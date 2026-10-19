//
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

// Package httpgrpcmux multiplexes a single TCP listener between a native gRPC
// server and an HTTP server, so one port can serve gRPC, gRPC-Web and plain
// HTTP at the same time.
//
// It implements the connection-level split described in
// docs/specs/issue-3628-single-endpoint.md, with the cmux library as the
// classifier (D-013). cmux reads the start of each new connection without
// consuming it and hands the connection to the gRPC or the HTTP listener. A
// service-owned matcher decides native gRPC from the first request's content
// type; every other connection (HTTP/1.x, HTTP/2 without gRPC, gRPC-Web) goes
// to the HTTP listener. A connection classified as gRPC carries only gRPC; a
// native gRPC request that arrives on a connection owned by the HTTP server is
// forwarded to the gRPC server by the HTTP handler.
//
// The matcher speaks only when a connection stays silent for the settings
// grace: gRPC clients (the CLI, Java) wait for the server's HTTP/2 settings
// frame before their first request. A connection the matcher has spoken to is
// committed to the gRPC side, because the client's acknowledgement of the
// injected frame reaches whichever server owns the connection, and net/http
// fails on an acknowledgement of settings it never sent. Every other client
// sends its first request without waiting and classifies unwritten-to.
//
// The mux implements kratos' transport.Server interface (Start/Stop with
// standard signatures) so it can join the application server lifecycle.
package httpgrpcmux

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/soheilhy/cmux"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// defaultClassifyTimeout bounds how long the matcher waits for a new
// connection's first request before the connection is closed or handed to the
// HTTP server. It applies to the classification step only: cmux clears it
// when the connection is handed over, so long-lived streams are not affected.
const defaultClassifyTimeout = 10 * time.Second

// defaultSettingsGrace is how long the matcher waits for the first request
// after the connection preface before it assumes the client waits for the
// server to speak first. gRPC clients (the CLI) wait for the server's
// HTTP/2 settings frame before they send their first request, so the matcher
// must send one. Every other client (browsers, plain HTTP/2) sends its first
// request without waiting and stays unspoken-to.
const defaultSettingsGrace = 250 * time.Millisecond

// defaultMaxClassifiedBytes bounds how much of a connection the matcher may
// read while looking for the first request headers. A connection that needs
// more goes to the HTTP server, whose own header limits dispose of it.
const defaultMaxClassifiedBytes = 512 << 10

// maxHpackString bounds a single decoded HPACK string in the matcher, so a
// small compressed block cannot expand into one huge header value.
const maxHpackString = 64 << 10

// hpackDynamicTableSize is the HPACK dynamic table size the matcher offers,
// the HTTP/2 spec default.
const hpackDynamicTableSize = 4096

// defaultMaxInflightClassifications caps how many connections may be in
// classification at the same time; beyond it, connections are shed.
const defaultMaxInflightClassifications = 8192

// acceptRetryDelay paces the retry loop after a recoverable accept error such
// as descriptor exhaustion.
const acceptRetryDelay = 5 * time.Millisecond

// closeDrainBound caps how long Close waits for the cmux serve loop to drain.
// Connections still in classification ride out their own window in the
// background instead of holding up shutdown.
const closeDrainBound = time.Second

// grpcContentTypePrefix matches every native gRPC content type, including
// application/grpc+proto and content types with parameters.
const grpcContentTypePrefix = "application/grpc"

// grpcWebContentTypePrefix is NOT part of the gRPC family above: gRPC-Web
// requests must reach the HTTP server, which owns the gRPC-Web wrapper.
const grpcWebContentTypePrefix = "application/grpc-web"

// headerContentType is the request header that carries the content type.
const headerContentType = "content-type"

// clientPreface is the HTTP/2 client preface, preallocated because the
// matcher compares it on every classified connection.
var clientPreface = []byte(http2.ClientPreface)

// Options configures a Mux.
type Options struct {
	// Logger receives classification debug information. Optional.
	Logger log.Logger
	// ClassifyTimeout bounds how long the matcher waits for a new
	// connection's first request. Defaults to 10s.
	ClassifyTimeout time.Duration
	// SettingsGrace is how long the matcher waits for the first request
	// before it sends the server's HTTP/2 settings frame to waiting
	// clients. Defaults to 250ms.
	SettingsGrace time.Duration
	// MaxInflightClassifications caps how many connections may be in
	// classification at the same time; beyond it, connections are shed.
	// Defaults to 8192.
	MaxInflightClassifications int
	// MaxClassifiedBytes bounds how much of a connection the matcher may
	// read. Defaults to 512 KiB.
	MaxClassifiedBytes int
}

// Mux splits one listener between a gRPC server and an HTTP server.
type Mux struct {
	// root wraps the raw listener with the stopping marker. cmux sub-listeners
	// embed the root listener, so whichever server stops first closes it for
	// both, and the marker lets every server tell that shutdown apart from a
	// fatal accept failure.
	root *stoppingListener
	cm   cmux.CMux

	grpcL net.Listener
	httpL net.Listener

	classifyTimeout    time.Duration
	settingsGrace      time.Duration
	maxClassifiedBytes int
	logger             *log.Helper
	// admit caps how many connections may be in classification at once. A
	// matcher holds a slot while it runs; a connection that cannot get one
	// matches no listener, and cmux closes it.
	admit chan struct{}
	// poisoned holds the connections the gRPC matcher has sent the server's
	// HTTP/2 settings frame to, but whose first request turned out not to be
	// native gRPC. Their acknowledgement of that frame is already recorded
	// in cmux's replay buffer, and net/http fails a connection that
	// acknowledges settings it never sent, so the HTTP matcher declines
	// them and cmux closes them. The gRPC matcher sets the entry before it
	// declines; the HTTP matcher, which cmux always runs next, clears it.
	poisoned sync.Map

	stopping atomic.Bool
	serveWG  sync.WaitGroup
	// serveOnce and closeOnce keep Start and Close idempotent: kratos may
	// call them more than once, and wire cleanup runs Close even when Start
	// never ran.
	serveOnce sync.Once
	closeOnce sync.Once
}

// stoppingListener marks its Mux as stopping before the listener closes.
type stoppingListener struct {
	net.Listener
	stopping *atomic.Bool
}

func (l *stoppingListener) Close() error {
	l.stopping.Store(true)
	return l.Listener.Close()
}

// New creates a mux over l. Both listeners are usable immediately;
// connections start flowing once Start is called.
func New(l net.Listener, opts *Options) *Mux {
	if opts == nil {
		opts = &Options{}
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.DefaultLogger
	}
	timeout := opts.ClassifyTimeout
	if timeout <= 0 {
		timeout = defaultClassifyTimeout
	}
	maxInflight := opts.MaxInflightClassifications
	if maxInflight <= 0 {
		maxInflight = defaultMaxInflightClassifications
	}
	maxBytes := opts.MaxClassifiedBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxClassifiedBytes
	}

	grace := opts.SettingsGrace
	if grace <= 0 {
		grace = defaultSettingsGrace
	}

	m := &Mux{
		classifyTimeout:    timeout,
		settingsGrace:      grace,
		maxClassifiedBytes: maxBytes,
		logger:             log.NewHelper(logger),
		admit:              make(chan struct{}, maxInflight),
	}
	m.root = &stoppingListener{Listener: l, stopping: &m.stopping}

	m.cm = cmux.New(m.root)
	// One read deadline bounds the whole classification step of a connection,
	// including the TLS handshake, which runs on the first read.
	m.cm.SetReadTimeout(timeout)

	// Registration order is match priority: the gRPC matcher runs first, so
	// an HTTP/2 connection with a native gRPC first request goes to the gRPC
	// listener, and everything else falls through to the HTTP listener. Both
	// are MatchWriters so they can reach the connection: the gRPC matcher
	// writes the server's settings frame to clients that wait for it, and
	// the HTTP matcher needs the connection to look up poisoned ones.
	m.grpcL = m.cm.MatchWithWriters(m.admitMatchWriter(m.matchNativeGRPC))
	m.httpL = m.cm.MatchWithWriters(m.admitMatchWriter(m.httpMatcher))

	m.cm.HandleError(func(err error) bool {
		var notMatched cmux.ErrNotMatched
		if errors.As(err, &notMatched) {
			// A connection that matched no listener was closed; routine.
			return true
		}
		if isAcceptRetryable(err) {
			m.logger.Debugf("mux accept error, retrying: %v", err)
			time.Sleep(acceptRetryDelay)
			return true
		}
		if errors.Is(err, net.ErrClosed) {
			// The root listener was closed: an orderly shutdown, whichever
			// path closed it.
			return false
		}
		// A fatal accept failure is not recoverable: log it loudly, the
		// process must be restarted.
		m.logger.Errorf("mux accept loop terminated: %v", err)
		return false
	})

	return m
}

// GRPC returns the listener that receives connections classified as native
// gRPC: HTTP/2 connections whose first request carries a gRPC content type.
func (m *Mux) GRPC() net.Listener { return m.grpcL }

// HTTP returns the listener that receives every other connection: HTTP/1.x,
// HTTP/2 without gRPC, and gRPC-Web.
func (m *Mux) HTTP() net.Listener { return m.httpL }

// Start runs the cmux serve loop until Stop is called.
func (m *Mux) Start(_ context.Context) error {
	m.serveOnce.Do(func() {
		m.serveWG.Add(1)
		go func() {
			defer m.serveWG.Done()
			// Serve returns when the root listener closes: an orderly
			// shutdown, or a fatal accept failure that the error handler
			// already logged. It then tears down both sub-listeners and
			// closes every connection that no server has accepted yet.
			if err := m.cm.Serve(); err != nil && !m.stopping.Load() {
				m.logger.Debugf("mux serve loop returned: %v", err)
			}
		}()
	})
	return nil
}

// Stop closes the underlying listener and both sub-listeners, unblocking any
// Accept call still in progress.
func (m *Mux) Stop(_ context.Context) error {
	m.Close()
	return nil
}

// Stopping reports whether Close has been called: a server fed by the mux can
// use it to tell an orderly shutdown apart from a fatal accept-loop failure.
func (m *Mux) Stopping() bool { return m.stopping.Load() }

// Close stops the mux and waits for the cmux serve loop to drain, bounded by
// closeDrainBound: connections still in classification ride out their own
// classification window in the background, and die with the process.
func (m *Mux) Close() {
	m.closeOnce.Do(func() {
		// Close the root listener first: Serve unblocks and tears down both
		// sub-listeners. cmux.Close then releases the connections that
		// already matched but were not accepted by a server.
		_ = m.root.Close()
		m.cm.Close()
		drained := make(chan struct{})
		go func() {
			m.serveWG.Wait()
			close(drained)
		}()
		select {
		case <-drained:
		case <-time.After(closeDrainBound):
		}
	})
}

// IsOrderlyShutdownErr reports whether err is the listener error of this
// mux's orderly shutdown rather than a fatal accept failure. Servers fed by
// the mux use it to keep a lost shutdown race from failing app startup: on a
// fatal accept failure the mux never stops, so the error still surfaces and
// the process restarts instead of serving a dead single port.
func (m *Mux) IsOrderlyShutdownErr(err error) bool {
	return m.stopping.Load() && (errors.Is(err, net.ErrClosed) ||
		errors.Is(err, cmux.ErrServerClosed) ||
		errors.Is(err, cmux.ErrListenerClosed))
}

// admitMatchWriter wraps a MatchWriter with classification admission control:
// it holds a slot while the matcher runs, and reports no match when the
// admission cap is reached, so cmux closes the connection.
func (m *Mux) admitMatchWriter(match cmux.MatchWriter) cmux.MatchWriter {
	return func(w io.Writer, r io.Reader) bool {
		select {
		case m.admit <- struct{}{}:
		default:
			return false
		}
		defer func() { <-m.admit }()
		return match(w, r)
	}
}

// isAcceptRetryable reports whether an accept error is recoverable: read
// timeouts, and descriptor exhaustion, which is temporary on Linux.
func isAcceptRetryable(err error) bool {
	return isTimeout(err) || errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE)
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// httpMatcher accepts every remaining connection that produced at least one
// byte and that the gRPC matcher did not speak to. A connection that produced
// none, or whose read failed, matches no listener and is closed by cmux.
func (m *Mux) httpMatcher(w io.Writer, r io.Reader) bool {
	// cmux passes the raw connection as the writer, and the gRPC matcher
	// keys poisoned connections by it. A connection that is not identifiable
	// cannot have been spoken to: the gRPC matcher declines before speaking
	// unless it can reach the connection too.
	if conn, ok := w.(net.Conn); ok {
		if _, poisoned := m.poisoned.LoadAndDelete(conn); poisoned {
			m.logger.Warnw("msg", "closing connection: the classifier sent its settings frame, but the first request was not native gRPC")
			return false
		}
	}
	// One byte proves the connection carries traffic. The read goes through
	// the sniffing reader, so cmux records it and replays it to the HTTP
	// server.
	var b [1]byte
	for {
		n, err := r.Read(b[:])
		if err != nil || n > 0 {
			return n > 0
		}
	}
}

// matchNativeGRPC reports whether the connection's first request carries a
// native gRPC content type.
//
// Clients that send their first request without waiting (browsers, plain
// HTTP/2) classify without a single byte written to them. A client that stays
// silent for the settings grace is a gRPC client waiting for the server's
// HTTP/2 settings frame — the gRPC Go client does this, and so do Java
// clients — so the matcher sends one. From that moment the connection is
// committed: the client's acknowledgement of the injected frame is already
// recorded in cmux's replay buffer, net/http fails a connection that
// acknowledges settings it never sent, and the gRPC server ignores extra
// acknowledgements. A committed connection whose first request is not native
// gRPC, or that never sends its request, is therefore declined and poisoned:
// the HTTP matcher declines it too, and cmux closes it.
//
// Frame parsing and HPACK decoding are the standard HTTP/2 library's;
// everything the matcher reads stays buffered in cmux and is replayed to the
// server that wins the connection. The matcher bounds how much of the
// connection it may read.
func (m *Mux) matchNativeGRPC(w io.Writer, r io.Reader) bool {
	// The extra byte makes any read past the cap fail instead of stopping
	// silently at it: a connection that needs more than the cap to classify
	// goes to the HTTP server, whose own limits dispose of it.
	lr := &io.LimitedReader{R: r, N: int64(m.maxClassifiedBytes) + 1}
	if !hasHTTP2Preface(lr) {
		// HTTP/1.x or anything else: the HTTP server produces a proper error
		// response for malformed traffic. Nothing was written.
		return false
	}

	conn, ok := w.(net.Conn)
	if !ok {
		return false
	}

	framer := http2.NewFramer(w, lr)
	var contentType string
	hdec := hpack.NewDecoder(hpackDynamicTableSize, func(hf hpack.HeaderField) {
		if strings.EqualFold(hf.Name, headerContentType) {
			contentType = hf.Value
		}
	})
	hdec.SetMaxStringLength(maxHpackString)

	// Narrow the read deadline cmux set to the classification window down to
	// the settings grace: how long to wait for a first request before
	// speaking. decline reports no match, and poisons the connection when the
	// matcher has already spoken, so it never reaches net/http.
	spoken := false
	_ = conn.SetReadDeadline(time.Now().Add(m.settingsGrace))
	decline := func() bool {
		if spoken {
			m.poisoned.Store(conn, struct{}{})
		}
		return false
	}
	speak := func() bool {
		// Bound the write, then give the committed classification a fresh
		// window.
		_ = conn.SetDeadline(time.Now().Add(m.classifyTimeout))
		if err := framer.WriteSettings(); err != nil {
			return false
		}
		spoken = true
		_ = conn.SetReadDeadline(time.Now().Add(m.classifyTimeout))
		return true
	}

	for {
		f, err := framer.ReadFrame()
		if err != nil {
			if !spoken && isTimeout(err) {
				// The client is waiting for the server to speak first.
				if !speak() {
					return decline()
				}
				continue
			}
			// Read failure, budget exhausted, or deadline: a committed
			// connection cannot go to the HTTP server.
			return decline()
		}
		// HEADERS and CONTINUATION are the only frames that carry header
		// block fragments; SETTINGS (including the acknowledgement of the
		// injected frame), PING, WINDOW_UPDATE, ... are skipped and stay in
		// the replay: the gRPC server ignores unsolicited acknowledgements.
		if hb, ok := f.(interface{ HeaderBlockFragment() []byte }); ok {
			if _, err := hdec.Write(hb.HeaderBlockFragment()); err != nil {
				return decline()
			}
			if f.Header().Flags&http2.FlagHeadersEndHeaders != 0 {
				if err := hdec.Close(); err != nil {
					return decline()
				}
				if !IsNativeGRPCContentType(contentType) {
					return decline()
				}
				return true
			}
		}
	}
}

// hasHTTP2Preface reports whether the connection starts with the HTTP/2 client
// preface. The preface is exact bytes, unlike header names and values. It
// reads byte ranges without over-consuming: everything it reads stays
// buffered in cmux and is replayed to the server.
func hasHTTP2Preface(r io.Reader) bool {
	buf := make([]byte, len(clientPreface))
	n := 0
	for n < len(clientPreface) {
		read, err := r.Read(buf[n:])
		n += read
		if err != nil {
			return false
		}
		if !bytes.HasPrefix(buf[:n], clientPreface[:n]) {
			return false
		}
	}
	return true
}

// IsNativeGRPCContentType reports whether the given content type identifies a
// native gRPC request, as opposed to a gRPC-Web or plain HTTP one. The HTTP
// server uses it to forward native gRPC requests that arrive on a connection
// the classifier handed to the HTTP side.
func IsNativeGRPCContentType(contentType string) bool {
	return hasFoldPrefix(contentType, grpcContentTypePrefix) &&
		!hasFoldPrefix(contentType, grpcWebContentTypePrefix)
}

// hasFoldPrefix reports whether s starts with prefix, ignoring case, without
// allocating a copy of s.
func hasFoldPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}
