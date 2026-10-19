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
// classifier (D-016). cmux reads the start of each new connection without
// consuming it and hands the connection to the gRPC or the HTTP listener. A
// service-owned matcher decides native gRPC from the first request's content
// type; every other connection (HTTP/1.x, HTTP/2 without gRPC, gRPC-Web) goes
// to the HTTP listener. A connection classified as gRPC carries only gRPC; a
// native gRPC request that arrives on a connection owned by the HTTP server is
// forwarded to the gRPC server by the HTTP handler.
//
// The matcher speaks only when a connection stays silent for the settings
// grace: gRPC clients (the CLI, Java) wait for the server's HTTP/2 settings
// frame before their first request. Ordinary HTTP/2 clients may also wait for
// this frame. When such a connection goes to HTTP, the matcher consumes its
// SETTINGS acknowledgement and the HTTP listener removes that one frame from
// cmux's replay. The HTTP server owns every subsequent SETTINGS exchange.
//
// The mux implements kratos' transport.Server interface (Start/Stop with
// standard signatures) so it can join the application server lifecycle.
package httpgrpcmux

import (
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
// server to speak first. Both native gRPC and ordinary HTTP/2 clients may
// wait for the server's SETTINGS before sending their first request.
const defaultSettingsGrace = 250 * time.Millisecond

// initialMaxFrameSize is the HTTP/2 frame size limit before the server has
// advertised a larger value. Bound allocation before reading frame payloads;
// a LimitedReader alone only bounds bytes consumed, not allocated buffers.
const initialMaxFrameSize = 1 << 14

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
	admit    chan struct{}
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

func (l *stoppingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &classificationConn{Conn: c, settingsACKOffset: -1}, nil
}

func (l *stoppingListener) Close() error {
	l.stopping.Store(true)
	return l.Listener.Close()
}

// classificationConn carries handshake state through cmux. Matchers update
// it before cmux publishes the connection to a sub-listener. No shared map
// or state remains after an unaccepted connection closes.
type classificationConn struct {
	net.Conn
	settingsACKOffset int64
	rejectHTTP        bool
}

// httpListener strips only the acknowledgement of the matcher's SETTINGS.
// Its exact offset is known before handoff, so serving HTTP does not require
// a second frame parser or any inspection of later requests or streams.
type httpListener struct{ net.Listener }

func (l httpListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	if mc, ok := c.(*cmux.MuxConn); ok {
		if state, ok := mc.Conn.(*classificationConn); ok && state.settingsACKOffset >= 0 {
			return &httpConn{Conn: c, beforeACK: state.settingsACKOffset}, nil
		}
	}
	return c, nil
}

type httpConn struct {
	net.Conn
	beforeACK int64
}

func (c *httpConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if c.beforeACK > 0 {
		n, err := c.Conn.Read(p[:min(int64(len(p)), c.beforeACK)])
		c.beforeACK -= int64(n)
		return n, err
	}
	if c.beforeACK == 0 {
		// The matcher already read and validated this complete, empty ACK.
		// It is entirely in cmux's replay, even when the caller reads one
		// byte at a time. Leave the next server SETTINGS/ACK pair untouched.
		var ack [9]byte
		_, err := io.ReadFull(c.Conn, ack[:])
		c.beforeACK = -1
		if err != nil {
			return 0, err
		}
	}
	return c.Conn.Read(p)
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
	// takes its admission slot itself, once the connection prefaced. The
	// HTTP chain rejects incomplete handshakes before it reads anything.
	m.grpcL = m.cm.MatchWithWriters(m.matchNativeGRPC)
	m.httpL = httpListener{m.cm.MatchWithWriters(m.rejectIncompleteHandshake(m.requireTraffic))}

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

// rejectIncompleteHandshake closes connections whose injected SETTINGS
// exchange failed. A completed HTTP handshake is safe to hand off because
// httpListener removes the acknowledgement belonging to the matcher.
func (m *Mux) rejectIncompleteHandshake(next cmux.MatchWriter) cmux.MatchWriter {
	return func(w io.Writer, r io.Reader) bool {
		if conn, ok := w.(*classificationConn); ok && conn.rejectHTTP {
			return false
		}
		return next(w, r)
	}
}

// requireTraffic is the terminal matcher of the HTTP chain: it accepts every
// remaining connection that produced at least one byte. A connection that
// produced none, or whose read failed, matches no listener and is closed by
// cmux. The one-byte read waits for the connection to carry traffic before
// the admission slot is taken: a silent connection buffers nothing and must
// not consume a slot — the slots bound buffered bytes, and the file descriptor
// limit bounds idle connections, as before this package existed.
func (m *Mux) requireTraffic(_ io.Writer, r io.Reader) bool {
	// cmux re-serves sniffed bytes between matcher sessions, never within
	// one, so no matcher may re-read what it already consumed: this is the
	// only read of the HTTP chain.
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return false
	}
	if !m.tryAcquire() {
		return false
	}
	defer func() { <-m.admit }()
	return true
}

// tryAcquire takes an admission slot when one is free, so cmux closes the
// connection when the classification cap is reached.
func (m *Mux) tryAcquire() bool {
	select {
	case m.admit <- struct{}{}:
		return true
	default:
		return false
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

// matchNativeGRPC reports whether the connection's first request carries a
// native gRPC content type.
//
// Clients that send their first request without waiting classify without a
// single byte written to them. After the settings grace, the matcher sends
// SETTINGS to unblock either kind of waiting HTTP/2 client. Before an HTTP
// handoff it records the ACK's exact replay offset, even if the ACK follows
// the first request. httpListener removes that ACK; native gRPC continues to
// receive the original replay. Incomplete injected handshakes are closed.
//
// Frame parsing and HPACK decoding are the standard HTTP/2 library's;
// everything the matcher reads stays buffered in cmux and is replayed to the
// server that wins the connection. The matcher bounds how much of the
// connection it may read.
func (m *Mux) matchNativeGRPC(w io.Writer, r io.Reader) bool {
	// The preface is read before any admission slot is taken: a silent
	// connection buffers nothing and must not consume one — the slots bound
	// buffered bytes, and the file descriptor limit bounds idle connections.
	// A non-prefaced connection (HTTP/1.x, garbage) declines here without
	// ever touching admission.
	//
	// The extra byte in the cap makes any read past it fail instead of
	// stopping silently at it: a connection that needs more than the cap to
	// classify goes to the HTTP server, whose own limits dispose of it.
	lr := &io.LimitedReader{R: r, N: int64(m.maxClassifiedBytes) + 1}
	if !matchHTTP2Preface(lr) {
		// HTTP/1.x or anything else: the HTTP server produces a proper error
		// response for malformed traffic. Nothing was written.
		return false
	}

	// The connection prefaced, so frames will buffer: take an admission
	// slot. A connection that cannot get one matches nothing, and cmux
	// closes it.
	if !m.tryAcquire() {
		return false
	}
	defer func() { <-m.admit }()

	conn, ok := w.(*classificationConn)
	if !ok {
		return false
	}

	framer := http2.NewFramer(w, lr)
	framer.SetMaxReadFrameSize(initialMaxFrameSize)
	var contentType string
	hdec := hpack.NewDecoder(hpackDynamicTableSize, func(hf hpack.HeaderField) {
		if strings.EqualFold(hf.Name, headerContentType) {
			contentType = hf.Value
		}
	})
	hdec.SetMaxStringLength(maxHpackString)

	// Narrow the read deadline cmux set to the classification window down to
	// the settings grace: how long to wait for a first request before
	// speaking. decline reports no match, and rejects an incomplete handshake
	// when the matcher has spoken, so it never reaches net/http. boundary is the
	// remaining read budget at the last completed frame: a timeout with a
	// smaller budget fired mid-frame. The rule is correct only while the
	// framer reads exactly frame-header plus payload with no read-ahead;
	// inserting a buffering reader in this chain breaks it.
	spoken := false
	firstRequest := false
	nativeGRPC := false
	boundary := lr.N
	_ = conn.SetReadDeadline(time.Now().Add(m.settingsGrace))
	decline := func() bool {
		if spoken {
			conn.rejectHTTP = true
		}
		return false
	}
	speak := func() bool {
		// The write is bounded on its own: cmux clears only the read
		// deadline when the connection is handed over, so a lingering write
		// deadline would kill long-lived streams downstream.
		_ = conn.SetWriteDeadline(time.Now().Add(m.classifyTimeout))
		// A failed handshake cannot be replayed to HTTP, even if only part
		// of the SETTINGS frame was delivered.
		spoken = true
		err := framer.WriteSettings()
		_ = conn.SetWriteDeadline(time.Time{})
		if err != nil {
			return false
		}
		_ = conn.SetReadDeadline(time.Now().Add(m.classifyTimeout))
		return true
	}

	for {
		offset := int64(m.maxClassifiedBytes) + 1 - lr.N
		f, err := framer.ReadFrame()
		if err != nil {
			if !spoken && isTimeout(err) {
				if lr.N < boundary {
					// The grace fired mid-frame: a slow sender, not a client
					// waiting for the server's settings. The framer cannot
					// resume a half-read frame, and speaking now would feed
					// it the rest of the frame as garbage. Decline without
					// speaking: the replay holds everything the client sent,
					// and the HTTP server serves the request through the
					// documented fallback (D-014).
					return false
				}
				// The client is waiting for the server to speak first.
				if !speak() {
					return decline()
				}
				continue
			}
			// Read failure, budget exhausted, or deadline: an incomplete
			// injected handshake cannot go to the HTTP server.
			return decline()
		}
		boundary = lr.N
		if sf, ok := f.(*http2.SettingsFrame); ok && sf.IsAck() && spoken && conn.settingsACKOffset < 0 {
			conn.settingsACKOffset = offset
		}
		// HEADERS and CONTINUATION are the only frames that carry header
		// block fragments; SETTINGS (including the acknowledgement of the
		// injected frame), PING, WINDOW_UPDATE, ... are skipped and stay in
		// the replay: the gRPC server ignores unsolicited acknowledgements.
		if hb, ok := f.(interface{ HeaderBlockFragment() []byte }); ok && !firstRequest {
			if _, err := hdec.Write(hb.HeaderBlockFragment()); err != nil {
				return decline()
			}
			if f.Header().Flags&http2.FlagHeadersEndHeaders != 0 {
				if err := hdec.Close(); err != nil {
					return decline()
				}
				firstRequest = true
				nativeGRPC = IsNativeGRPCContentType(contentType)
			}
		}
		if firstRequest {
			if nativeGRPC {
				return true
			}
			if !spoken || conn.settingsACKOffset >= 0 {
				return false
			}
		}
	}
}

// matchHTTP2Preface is cmux's HTTP/2 preface matcher: it reports whether the
// connection starts with the exact HTTP/2 client preface bytes, without
// over-consuming. Everything it reads stays buffered in cmux and is replayed
// to the server.
var matchHTTP2Preface = cmux.HTTP2()

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
