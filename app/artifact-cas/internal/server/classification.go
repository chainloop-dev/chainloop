// Copyright 2026 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"errors"
	"net"
	"sync"
	"time"
)

const (
	classificationTimeout     = 10 * time.Second
	maxClassifyingConnections = 128
	maxClassificationBytes    = 64 << 10
)

// classificationListener owns sockets only until cmux hands them to a server.
// cmux clears the read deadline at that boundary (SetReadTimeout must be set).
// The timer also bounds TLS handshakes and SETTINGS writes, and Close releases
// pending sockets without interrupting streams already owned by HTTP or gRPC.
type classificationListener struct {
	net.Listener
	mu      sync.Mutex
	pending map[*classificationConn]*time.Timer
	closed  bool
	timeout time.Duration
}

func (l *classificationListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		l.mu.Lock()
		if l.closed {
			l.mu.Unlock()
			_ = conn.Close()
			return nil, net.ErrClosed
		}
		if len(l.pending) >= maxClassifyingConnections {
			l.mu.Unlock()
			_ = conn.Close()
			continue
		}
		c := &classificationConn{Conn: conn, owner: l, remaining: maxClassificationBytes}
		l.pending[c] = time.AfterFunc(l.timeout, func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			if _, pending := l.pending[c]; pending {
				delete(l.pending, c)
				_ = c.Conn.Close()
			}
		})
		l.mu.Unlock()
		return c, nil
	}
}

func (l *classificationListener) Close() error {
	l.mu.Lock()
	l.closed = true
	for conn, timer := range l.pending {
		timer.Stop()
		_ = conn.Conn.Close()
		delete(l.pending, conn)
	}
	l.mu.Unlock()
	return l.Listener.Close()
}

type classificationConn struct {
	net.Conn
	owner     *classificationListener
	remaining int
}

func (c *classificationConn) Read(p []byte) (int, error) {
	// Only the cmux goroutine reads or changes remaining until handoff.
	if c.remaining == 0 {
		_ = c.Close()
		return 0, errors.New("CAS connection classification exceeded its byte limit")
	}
	if c.remaining > 0 && len(p) > c.remaining {
		p = p[:c.remaining]
	}
	n, err := c.Conn.Read(p)
	if c.remaining > 0 {
		c.remaining -= n
	}
	return n, err
}

func (c *classificationConn) SetReadDeadline(deadline time.Time) error {
	if !deadline.IsZero() {
		return c.Conn.SetReadDeadline(deadline)
	}
	c.owner.mu.Lock()
	defer c.owner.mu.Unlock()
	if timer, pending := c.owner.pending[c]; pending {
		timer.Stop()
		delete(c.owner.pending, c)
		c.remaining = -1
	}
	return c.Conn.SetReadDeadline(deadline)
}

func (c *classificationConn) Close() error {
	c.owner.mu.Lock()
	if timer, pending := c.owner.pending[c]; pending {
		timer.Stop()
		delete(c.owner.pending, c)
	}
	c.owner.mu.Unlock()
	return c.Conn.Close()
}
