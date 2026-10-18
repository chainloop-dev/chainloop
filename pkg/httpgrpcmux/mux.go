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
// docs/specs/issue-3628-single-endpoint.md: the mux looks at the start of each
// new connection without consuming it. An HTTP/2 connection whose first request
// carries a gRPC content type is handed to the gRPC listener; every other
// connection (HTTP/1.x, HTTP/2 without gRPC, gRPC-Web) goes to the HTTP
// listener. A connection classified as gRPC carries only gRPC; a native gRPC
// request that arrives on a connection owned by the HTTP server is forwarded
// to the gRPC server by the HTTP handler.
//
// The mux implements kratos' transport.Server interface (Start/Stop with
// standard signatures) so it can join the application server lifecycle.
package httpgrpcmux

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"golang.org/x/net/http2/hpack"
)

// clientConnPreface is the connection preface that every HTTP/2 client sends
// right after the connection is established.
const clientConnPreface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"

// frameHeaderLen is the fixed size of an HTTP/2 frame header.
const frameHeaderLen = 9

// defaultClassifyTimeout bounds how long the classifier waits for a new
// connection's first request before dropping it. It applies to the
// classification step only, and is cleared before the connection is handed
// over, so long-lived streams are not affected.
const defaultClassifyTimeout = 10 * time.Second

// defaultSettingsGrace is how long the classifier waits for the client's first
// HTTP/2 frame before assuming the client is waiting for the server to speak
// first, and sending an empty SETTINGS frame to unblock it.
const defaultSettingsGrace = time.Second

// maxClassifiedBytes bounds how much of a connection the classifier may read
// while looking for the first request headers. Anything beyond that is
// considered malformed and the connection is dropped.
const maxClassifiedBytes = 512 << 10

// maxHeaderBlockSize bounds a single HTTP/2 header block (plus its
// CONTINUATION frames). Real gRPC requests stay far below it.
const maxHeaderBlockSize = 1 << 20

// Decoded-header-list limits: a small compressed block can otherwise expand
// into a very large header list (HPACK amplification).
const (
	maxDecodedFields = 1024
	maxDecodedBytes  = 256 << 10
	maxHpackString   = 64 << 10
)

// maxClassifiedFrames bounds how many HTTP/2 frames the classifier processes
// before the first request.
const maxClassifiedFrames = 4096

// defaultMaxInflightClassifications caps how many connections may be in
// classification at the same time; beyond it, connections are shed.
const defaultMaxInflightClassifications = 8192

// maxTotalClassifiedBytes bounds the aggregate bytes recorded across all
// connections in classification at the same time, so the per-connection budget
// times the connection cap cannot multiply into unbounded memory.
const maxTotalClassifiedBytes = 256 << 20

// errTotalBudgetExceeded is returned when the aggregate classification budget
// is exhausted; the connection is dropped.
var errTotalBudgetExceeded = errors.New("aggregate classification budget exceeded")

// maxAcceptBackoff caps the retry delay after a recoverable accept error.
const maxAcceptBackoff = time.Second

// grpcContentTypePrefix matches every native gRPC content type, including
// application/grpc+proto and content types with parameters.
const grpcContentTypePrefix = "application/grpc"

// grpcWebContentTypePrefix is NOT part of the gRPC family above: gRPC-Web
// requests must reach the HTTP server, which owns the gRPC-Web wrapper.
const grpcWebContentTypePrefix = "application/grpc-web"

// HTTP/2 frame types used by the classifier.
const (
	http2FrameHeaders      = 0x1
	http2FramePushPromise  = 0x5
	http2FrameSettings     = 0x4
	http2FrameContinuation = 0x9
)

// HTTP/2 frame flags used by the classifier.
const (
	http2FlagEndHeaders = 0x4
	http2FlagPadded     = 0x8
	http2FlagPriority   = 0x20
	// http2FlagSettingsAck marks a SETTINGS frame that acknowledges the peer's
	// settings.
	http2FlagSettingsAck = 0x1
)

// http2ServerEmptySettings is an empty SETTINGS frame. When a client stalls
// waiting for the server to speak first, the classifier sends it to unblock
// the client so it can read the first request.
var http2ServerEmptySettings = []byte{0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00}

// acceptBacklog is the size of each virtual listener's delivery queue.
const acceptBacklog = 16

// Options configures a Mux.
type Options struct {
	// Logger receives classification debug information. Optional.
	Logger log.Logger
	// ClassifyTimeout bounds how long the classifier waits for a new
	// connection's first request. Defaults to 10s.
	ClassifyTimeout time.Duration
	// SettingsGrace is how long the classifier waits for the client's first
	// HTTP/2 frame before assuming it waits for the server to speak first,
	// and sending an empty SETTINGS frame to unblock it. Defaults to 1s.
	SettingsGrace time.Duration
	// MaxInflightClassifications caps how many connections may be in
	// classification at the same time; beyond it, connections are shed.
	// Defaults to 8192.
	MaxInflightClassifications int
	// MaxTotalClassifiedBytes bounds the aggregate bytes recorded across all
	// connections in classification at the same time. Defaults to 256 MiB.
	MaxTotalClassifiedBytes int64
}

// Mux splits one listener between a gRPC server and an HTTP server.
type Mux struct {
	raw net.Listener

	grpcL *virtualListener
	httpL *virtualListener

	classifyTimeout time.Duration
	settingsGrace   time.Duration
	logger          *log.Helper
	// admit caps how many connections may be in classification at once.
	admit chan struct{}
	// stopping is set before Close tears the listeners down, so servers can
	// distinguish an orderly shutdown from a fatal accept-loop failure.
	stopping atomic.Bool
	// inflight tracks the connections that are being classified, so Close
	// can release the ones still parked on the classification deadline.
	mu       sync.Mutex
	inflight map[net.Conn]struct{}
	// budget tracks the aggregate bytes charged across in-flight
	// classifications: the bytes recorded so far, plus the payload
	// reservations held while a frame payload is being read; budgetLimit is
	// its cap.
	budget      *atomic.Int64
	budgetLimit int64

	wg        sync.WaitGroup
	closeOnce sync.Once
}

// New creates a mux over l. Both virtual listeners are usable immediately;
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
	grace := opts.SettingsGrace
	if grace <= 0 {
		grace = defaultSettingsGrace
	}
	maxInflight := opts.MaxInflightClassifications
	if maxInflight <= 0 {
		maxInflight = defaultMaxInflightClassifications
	}
	totalBytes := opts.MaxTotalClassifiedBytes
	if totalBytes <= 0 {
		totalBytes = maxTotalClassifiedBytes
	}
	return &Mux{
		raw:             l,
		grpcL:           newVirtualListener(l.Addr()),
		httpL:           newVirtualListener(l.Addr()),
		classifyTimeout: timeout,
		settingsGrace:   grace,
		admit:           make(chan struct{}, maxInflight),
		inflight:        make(map[net.Conn]struct{}),
		budget:          &atomic.Int64{},
		budgetLimit:     totalBytes,
		logger:          log.NewHelper(logger),
	}
}

// GRPC returns the listener that receives connections classified as native
// gRPC: HTTP/2 connections whose first request carries a gRPC content type.
func (m *Mux) GRPC() net.Listener { return m.grpcL }

// HTTP returns the listener that receives every other connection: HTTP/1.x,
// HTTP/2 without gRPC, and gRPC-Web.
func (m *Mux) HTTP() net.Listener { return m.httpL }

// Start runs the accept loop until Stop is called.
func (m *Mux) Start(_ context.Context) error {
	m.wg.Add(1)
	go m.acceptLoop()
	return nil
}

// Stop closes the underlying listener and both virtual listeners, unblocking
// any Accept call still in progress.
func (m *Mux) Stop(_ context.Context) error {
	m.Close()
	return nil
}

// Stopping reports whether Close has been called: servers fed by the mux use
// it to tell an orderly shutdown apart from a fatal accept-loop failure.
func (m *Mux) Stopping() bool { return m.stopping.Load() }

// Close stops the mux and waits for the accept loop to drain.
func (m *Mux) Close() {
	m.closeOnce.Do(func() {
		m.stopping.Store(true)
		_ = m.raw.Close()
		// Close the virtual listeners even when Start never ran: the accept
		// loop would otherwise be the only place doing it, and their
		// dispatchers would keep running with queued connections inside.
		_ = m.grpcL.Close()
		_ = m.httpL.Close()
		// Release the connections still parked in classification: closing
		// them unblocks their goroutines, so the wait below is fast instead
		// of riding out the classification deadline.
		m.mu.Lock()
		for conn := range m.inflight {
			_ = conn.Close()
		}
		m.mu.Unlock()
		m.wg.Wait()
	})
}

func (m *Mux) acceptLoop() {
	defer m.wg.Done()
	var backoff time.Duration
	for {
		conn, err := m.raw.Accept()
		if err != nil {
			// Descriptor exhaustion and timeouts are recoverable: retry with
			// backoff instead of killing the accept loop.
			if isAcceptRetryable(err) {
				if backoff == 0 {
					backoff = 5 * time.Millisecond
				} else if backoff < maxAcceptBackoff {
					backoff *= 2
				}
				time.Sleep(backoff)
				continue
			}
			// Listener closed or fatal error: the accept loop is gone. Log it loudly —
			// the servers notice through the closed virtual listeners, and the process
			// must be restarted (a fatal accept failure is not recoverable).
			m.logger.Errorf("mux accept loop terminated: %v", err)
			_ = m.grpcL.Close()
			_ = m.httpL.Close()
			return
		}
		backoff = 0
		// Shed connections beyond the admission limit: the peer retries.
		select {
		case m.admit <- struct{}{}:
		default:
			m.logger.Debugf("shedding connection: classification admission limit reached")
			_ = conn.Close()
			continue
		}
		// Track the connection BEFORE the goroutine launches, so Close cannot
		// walk past it: registering inside classify would leave a window where
		// Close misses the accepted connection and rides out its full
		// classification deadline. Ownership transfers to the server at
		// handover, after which the entry is removed.
		m.mu.Lock()
		m.inflight[conn] = struct{}{}
		m.mu.Unlock()
		m.wg.Add(1)
		go func() {
			defer func() {
				m.mu.Lock()
				delete(m.inflight, conn)
				m.mu.Unlock()
				<-m.admit
			}()
			// classify owns the WaitGroup accounting for this goroutine.
			m.classify(conn)
		}()
	}
}

// isAcceptRetryable reports whether an accept error is recoverable: read
// timeouts, and descriptor exhaustion, which is temporary on Linux.
func isAcceptRetryable(err error) bool {
	return isTimeout(err) || errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE)
}

// classify looks at the start of a new connection and hands it to the right
// server. It never consumes more than what it replays to the server. The
// caller registered the connection in the inflight set before launching it
// here; it is removed when this goroutine ends.
func (m *Mux) classify(conn net.Conn) {
	defer m.wg.Done()

	bc := &replayConn{Conn: conn, recording: true, budget: m.budget, budgetLimit: m.budgetLimit}
	// One absolute deadline bounds the whole classification step: reads and
	// writes (the TLS handshake and the injected SETTINGS frame included).
	// It is cleared before the connection is handed over, so long-lived
	// streams are not affected.
	classifyEnd := time.Now().Add(m.classifyTimeout)
	_ = bc.SetDeadline(classifyEnd)
	// Release this connection's share of the aggregate budget when the
	// classification ends, whichever way it ends.
	defer func() {
		if bc.appended > 0 {
			bc.budget.Add(-bc.appended)
		}
	}()

	if tc, ok := conn.(*tls.Conn); ok {
		if err := tc.Handshake(); err != nil {
			m.logger.Debugf("dropping connection: TLS handshake failed: %v", err)
			_ = conn.Close()
			return
		}
		switch tc.ConnectionState().NegotiatedProtocol {
		case "", "http/1.1":
			// HTTP/1.1 was negotiated: no HTTP/2 frames to parse.
			m.handOver(bc, false)
			return
		}
		// "h2" or another protocol: sniff the first request below.
	}

	isGRPC, err := m.sniff(bc, classifyEnd)
	if err != nil {
		m.logger.Debugf("dropping connection: could not classify: %v", err)
		_ = conn.Close()
		return
	}
	m.handOver(bc, isGRPC)
}

// handOver clears the classification deadline and delivers the connection to
// the right virtual listener, replaying everything the classifier consumed.
func (m *Mux) handOver(bc *replayConn, isGRPC bool) {
	_ = bc.SetDeadline(time.Time{})
	if isGRPC {
		m.grpcL.deliver(bc.handoff())
	} else {
		m.httpL.deliver(bc.handoff())
	}
}

// sniff decides whether the connection's first request is native gRPC.
func (m *Mux) sniff(bc *replayConn, classifyEnd time.Time) (bool, error) {
	isH2, err := readClientPreface(bc)
	if err != nil {
		return false, err
	}
	if !isH2 {
		// HTTP/1.x or anything else. The HTTP server produces a proper
		// error response for malformed traffic.
		return false, nil
	}

	// A client that waits for the server to speak first stalls in the frame
	// loop below; sniffFirstRequest unblocks it after a short grace period.
	return m.sniffFirstRequest(bc, classifyEnd)
}

// readClientPreface reports whether the connection starts with the HTTP/2
// client preface. It reads byte ranges without over-consuming: everything it
// reads stays buffered in bc and is replayed to the server.
func readClientPreface(bc *replayConn) (bool, error) {
	preface := []byte(clientConnPreface)
	buf := make([]byte, len(preface))
	n := 0
	for n < len(preface) {
		read, err := bc.Read(buf[n:])
		if err != nil {
			return false, err
		}
		n += read
		if !bytes.HasPrefix(buf[:n], preface[:n]) {
			return false, nil
		}
	}
	return true, nil
}

// sniffFirstRequest walks HTTP/2 frames until the first HEADERS block is
// complete, then reports whether its content type is a native gRPC one.
//
// A client that waits for the server to speak first before sending anything
// stalls here. After a short grace period the classifier sends an empty
// SETTINGS frame to unblock it, and removes the SETTINGS-ACK that the client
// then sends for it from the replay, so the downstream server only ever sees
// ACKs for its own SETTINGS. The ACK may also arrive after the first request
// — a client whose frame-read loop races its request-write goroutine can send
// HEADERS first — in which case the classifier keeps consuming frames until
// it shows up (maybeWaitForSettingsAck). All reads stay bounded by
// classifyEnd, the absolute end of the classification window.
// http2FrameHeader is the decoded fixed part of an HTTP/2 frame.
type http2FrameHeader struct {
	length    int
	frameType byte
	flags     byte
	streamID  uint32
}

// readFrameHeader reads and decodes the fixed part of the next HTTP/2 frame.
func readFrameHeader(bc *replayConn, injected *bool, classifyEnd time.Time) (http2FrameHeader, error) {
	var hdr [frameHeaderLen]byte
	if err := readInto(bc, hdr[:], injected, classifyEnd); err != nil {
		return http2FrameHeader{}, err
	}
	return http2FrameHeader{
		length:    int(hdr[0])<<16 | int(hdr[1])<<8 | int(hdr[2]),
		frameType: hdr[3],
		flags:     hdr[4],
		streamID:  uint32(hdr[5])<<24 | uint32(hdr[6])<<16 | uint32(hdr[7])<<8 | uint32(hdr[8]),
	}, nil
}

// consumeSettingsAck validates a SETTINGS-ACK (zero payload, zero stream) and
// removes its bytes from the replay: the downstream server never expects the
// ACK of the injected SETTINGS.
func consumeSettingsAck(bc *replayConn, hdr http2FrameHeader) error {
	if hdr.length != 0 || hdr.streamID != 0 {
		return errors.New("malformed SETTINGS ACK")
	}
	bc.dropLast(frameHeaderLen)
	return nil
}

// checkClassificationBounds enforces the frame-count and byte caps shared by
// both classification loops.
func checkClassificationBounds(bc *replayConn, frames int) error {
	if frames > maxClassifiedFrames {
		return errors.New("too many frames before the first request")
	}
	if bc.recorded() > maxClassifiedBytes {
		return errors.New("classification buffer limit exceeded")
	}
	return nil
}

func (m *Mux) sniffFirstRequest(bc *replayConn, classifyEnd time.Time) (bool, error) {
	// Only the content-type decision is retained, under decoded-list caps: a
	// small compressed block can otherwise expand into a huge header list.
	counters := &headerCounters{}
	dec := hpack.NewDecoder(4096, counters.observe)
	dec.SetMaxStringLength(maxHpackString)

	var block []byte
	var inBlock bool
	// The client gets a short window to send its first frame on its own. The
	// classification deadline stays in force as the upper bound.
	graceEnd := time.Now().Add(m.settingsGrace)
	if graceEnd.After(classifyEnd) {
		graceEnd = classifyEnd
	}
	_ = bc.SetDeadline(graceEnd)
	injected := false
	// ackConsumed tracks whether the ACK of the injected SETTINGS was already
	// consumed. It only matters once injected is true: until then any ACK is
	// rejected below as unexpected traffic.
	ackConsumed := false
	frames := 0
	for {
		frames++
		if err := checkClassificationBounds(bc, frames); err != nil {
			return false, err
		}

		hdr, err := readFrameHeader(bc, &injected, classifyEnd)
		if err != nil {
			return false, err
		}

		if hdr.frameType == http2FrameSettings && hdr.flags&http2FlagSettingsAck != 0 {
			// A SETTINGS-ACK can only acknowledge the injected SETTINGS: no
			// server SETTINGS were sent before the handover. Anything else,
			// including an ACK with a payload or on a stream, is invalid and
			// would desynchronize the frame stream if replayed.
			if !injected {
				return false, errors.New("unexpected SETTINGS ACK before any injected SETTINGS")
			}
			if err := consumeSettingsAck(bc, hdr); err != nil {
				return false, err
			}
			ackConsumed = true
			continue
		}

		// Every payload read must fit in the classification budget.
		if bc.recorded()+hdr.length > maxClassifiedBytes {
			return false, errors.New("frame payload exceeds classification buffer limit")
		}

		switch hdr.frameType {
		case http2FrameHeaders:
			if inBlock {
				return false, errors.New("unexpected HEADERS inside a header block")
			}
			if hdr.length > maxHeaderBlockSize {
				return false, errors.New("HEADERS frame too large")
			}
			payload, err := readFramePayload(bc, hdr.length, &injected, classifyEnd)
			if err != nil {
				return false, err
			}
			frag, err := headerBlockFragment(payload, hdr.flags)
			if err != nil {
				return false, err
			}
			block = append(block, frag...)
			if hdr.flags&http2FlagEndHeaders == 0 {
				inBlock = true
				continue
			}
			return finishHeaderBlock(bc, dec, counters, block, injected, ackConsumed, classifyEnd)
		case http2FrameContinuation:
			if !inBlock {
				return false, errors.New("unexpected CONTINUATION frame")
			}
			if len(block)+hdr.length > maxHeaderBlockSize {
				return false, errors.New("header block too large")
			}
			payload, err := readFramePayload(bc, hdr.length, &injected, classifyEnd)
			if err != nil {
				return false, err
			}
			block = append(block, payload...)
			if hdr.flags&http2FlagEndHeaders == 0 {
				continue
			}
			return finishHeaderBlock(bc, dec, counters, block, injected, ackConsumed, classifyEnd)
		case http2FramePushPromise:
			// Only servers may push; a client PUSH_PROMISE is a protocol error.
			return false, errors.New("unexpected PUSH_PROMISE frame")
		default:
			// SETTINGS, WINDOW_UPDATE, PING, GOAWAY, ...: read the payload
			// (it must be replayed to the server) within the budget checked
			// above.
			if err := skipFramePayload(bc, hdr.length, &injected, classifyEnd); err != nil {
				return false, err
			}
		}
	}
}

// finishHeaderBlock decodes a completed header block, decides gRPC-ness and,
// for HTTP-classified connections with an outstanding injected SETTINGS, waits
// for its ACK before handover.
func finishHeaderBlock(bc *replayConn, dec *hpack.Decoder, counters *headerCounters, block []byte, injected, ackConsumed bool, classifyEnd time.Time) (bool, error) {
	isGRPC, err := contentTypeFromBlock(dec, counters, block)
	if err != nil {
		return false, err
	}
	if err := maybeWaitForSettingsAck(bc, isGRPC, injected, ackConsumed, classifyEnd); err != nil {
		return false, err
	}
	return isGRPC, nil
}

// maxSettingsAckWait bounds how long the classifier waits for the ACK of the
// injected SETTINGS when it arrives after the first request. Well below the
// classification window: a healthy client acknowledges immediately.
const maxSettingsAckWait = 2 * time.Second

// maybeWaitForSettingsAck removes the client's ACK of the injected SETTINGS
// from the replay even when it arrives after the first request: a client
// whose frame-read loop races its request-write goroutine can send HEADERS
// first and the ACK afterwards. If the late ACK were replayed, the downstream
// server would see two ACKs for one SETTINGS and fail with PROTOCOL_ERROR.
//
// The wait only applies to HTTP-classified connections: net/http breaks on
// the extra ACK, while the gRPC server ignores it, and waiting on a
// gRPC-classified connection would record large attestation DATA frames into
// the replay prefix, hitting the per-connection cap.
//
// While waiting, every frame read stays recorded in the replay prefix, so
// whatever the client sends in the meantime reaches the downstream server. If
// the ACK does not arrive in time the connection is dropped: the HTTP/2
// connection would break on the extra ACK anyway.
func maybeWaitForSettingsAck(bc *replayConn, isGRPC, injected, ackConsumed bool, classifyEnd time.Time) error {
	if isGRPC || !injected || ackConsumed {
		return nil
	}

	// Bound the wait by the classification window when the wait bound does
	// not fit inside it.
	waitEnd := time.Now().Add(maxSettingsAckWait)
	if waitEnd.After(classifyEnd) {
		waitEnd = classifyEnd
	}
	_ = bc.SetDeadline(waitEnd)
	// The SETTINGS were already injected: a timeout past waitEnd must drop
	// the connection, not inject a second time.
	injected = true
	frames := 0
	for {
		frames++
		if err := checkClassificationBounds(bc, frames); err != nil {
			return err
		}

		hdr, err := readFrameHeader(bc, &injected, classifyEnd)
		if err != nil {
			return fmt.Errorf("waiting for the SETTINGS ACK of the injected SETTINGS: %w", err)
		}

		if hdr.frameType == http2FrameSettings && hdr.flags&http2FlagSettingsAck != 0 {
			if err := consumeSettingsAck(bc, hdr); err != nil {
				return err
			}
			return nil
		}
		if bc.recorded()+hdr.length > maxClassifiedBytes {
			return errors.New("frame payload exceeds classification buffer limit")
		}
		// Any other frame (HEADERS, DATA, WINDOW_UPDATE, ...): skip it; its
		// bytes stay recorded and are replayed to the server.
		if err := skipFramePayload(bc, hdr.length, &injected, classifyEnd); err != nil {
			return err
		}
	}
}

// readInto fills buf from bc, keeping the bytes read before an error. A
// timeout while the client is still waiting for the server to speak first (no
// SETTINGS injected yet) unblocks it: the deadline is raised to classifyEnd,
// an empty SETTINGS frame is written, injected is marked accordingly, and the
// read continues with the partial progress intact. Any other error is
// returned as is.
func readInto(bc *replayConn, buf []byte, injected *bool, classifyEnd time.Time) error {
	n := 0
	for n < len(buf) {
		read, err := bc.Read(buf[n:])
		n += read
		if err == nil {
			continue
		}
		if isTimeout(err) && !*injected {
			// The client is waiting for the server to speak first. Raise the
			// deadline to the classification end BEFORE writing, so the write
			// itself is bounded by it.
			_ = bc.SetDeadline(classifyEnd)
			if _, err := bc.Write(http2ServerEmptySettings); err != nil {
				return err
			}
			*injected = true
			continue
		}
		// A Read may legally return data together with an error (io.Reader
		// contract): account the data above, and if the buffer is complete
		// the error is irrelevant — a trailing EOF after a full payload, for
		// instance. A budget failure returns (0, err) and still propagates.
		if n == len(buf) {
			return nil
		}
		return err
	}
	return nil
}

// skipFramePayloadLen is the size of the scratch buffer used to skip frame
// payloads that need no parsing.
const skipFramePayloadLen = 1024

// skipFramePayload reads and discards a frame payload of the given length. It
// reads into a small scratch buffer instead of io.CopyN, which discards the
// read error whenever the full count arrives — including an aggregate-budget
// failure delivered together with the last bytes of the payload.
func skipFramePayload(bc *replayConn, length int, injected *bool, classifyEnd time.Time) error {
	var scratch [skipFramePayloadLen]byte
	for remaining := length; remaining > 0; {
		chunk := scratch[:min(remaining, len(scratch))]
		if err := readInto(bc, chunk, injected, classifyEnd); err != nil {
			return err
		}
		remaining -= len(chunk)
	}
	return nil
}

// readFramePayload reads a frame payload of the given length. The allocation
// is charged against the aggregate budget before it happens: the recorded
// bytes alone would otherwise let every in-flight classification allocate up
// to maxClassifiedBytes without ever contributing to the budget. The charge
// is released when the read completes, whatever its outcome: by then Read has
// recorded — and charged — the payload bytes themselves, so only they remain.
func readFramePayload(bc *replayConn, length int, injected *bool, classifyEnd time.Time) ([]byte, error) {
	if bc.budget != nil {
		if bc.budget.Add(int64(length)) > bc.budgetLimit {
			bc.budget.Add(-int64(length))
			return nil, errTotalBudgetExceeded
		}
		// Release the reservation on every path, including the error returns
		// below: the recorded bytes are charged by Read itself.
		defer bc.budget.Add(-int64(length))
	}
	payload := make([]byte, length)
	// readInto, unlike io.ReadFull, never swallows a read error that arrives
	// together with the last bytes of the payload.
	if err := readInto(bc, payload, injected, classifyEnd); err != nil {
		return nil, err
	}
	return payload, nil
}

// headerBlockFragment strips the padding and the priority fields from a
// HEADERS frame payload.
func headerBlockFragment(payload []byte, flags byte) ([]byte, error) {
	if flags&http2FlagPadded != 0 {
		if len(payload) == 0 {
			return nil, errors.New("truncated HEADERS frame")
		}
		pad := int(payload[0])
		payload = payload[1:]
		if pad > len(payload) {
			return nil, errors.New("invalid HEADERS padding")
		}
		payload = payload[:len(payload)-pad]
	}
	if flags&http2FlagPriority != 0 {
		if len(payload) < 5 {
			return nil, errors.New("truncated HEADERS frame")
		}
		payload = payload[5:]
	}
	return payload, nil
}

// headerCounters tracks the decoded header list under its caps; only the
// content-type decision is retained.
type headerCounters struct {
	fields      int
	decodedSize int
	contentType string
}

func (c *headerCounters) observe(f hpack.HeaderField) {
	c.fields++
	c.decodedSize += len(f.Name) + len(f.Value)
	if strings.EqualFold(f.Name, "content-type") {
		c.contentType = f.Value
	}
}

// contentTypeFromBlock decodes an HTTP/2 header block and reports whether its
// content type belongs to the native gRPC family.
func contentTypeFromBlock(dec *hpack.Decoder, counters *headerCounters, block []byte) (bool, error) {
	if _, err := dec.Write(block); err != nil {
		return false, err
	}
	if err := dec.Close(); err != nil {
		return false, err
	}
	if counters.fields > maxDecodedFields || counters.decodedSize > maxDecodedBytes {
		return false, errors.New("decoded header list exceeds limits")
	}
	return isGRPCContentType(counters.contentType), nil
}

// isGRPCContentType reports whether the content type belongs to the native
// gRPC family. gRPC-Web content types start with the same prefix but belong
// to the HTTP server instead.
func isGRPCContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.HasPrefix(ct, grpcContentTypePrefix) &&
		!strings.HasPrefix(ct, grpcWebContentTypePrefix)
}

// IsNativeGRPCContentType reports whether the given content type identifies a
// native gRPC request, as opposed to a gRPC-Web or plain HTTP one. The HTTP
// server uses it to forward native gRPC requests that arrive on a connection
// the classifier handed to the HTTP side.
func IsNativeGRPCContentType(contentType string) bool {
	return isGRPCContentType(contentType)
}

// replayConn is used in two modes. During classification (replay=false) it
// reads from the live connection and records every byte it consumes. The
// handoff conn produced by handoff() (replay=true) first replays those bytes
// to the server, then streams the rest of the connection.
type replayConn struct {
	net.Conn

	prefix    []byte
	recording bool
	replay    bool
	pos       int
	// budget counts this connection's share of the mux's aggregate
	// classification budget; appended is its total recorded bytes so the
	// share can be released exactly.
	budget      *atomic.Int64
	budgetLimit int64
	appended    int64
}

func (c *replayConn) Read(p []byte) (int, error) {
	if c.replay && c.pos < len(c.prefix) {
		n := copy(p, c.prefix[c.pos:])
		c.pos += n
		if c.pos == len(c.prefix) {
			// The replay is done: release the buffer for the rest of the
			// connection's lifetime.
			c.prefix = nil
		}
		return n, nil
	}
	n, err := c.Conn.Read(p)
	if c.recording && n > 0 {
		c.prefix = append(c.prefix, p[:n]...)
		c.appended += int64(n)
		if c.budget != nil {
			if c.budget.Add(int64(n)) > c.budgetLimit {
				// The aggregate budget is exhausted: give the bytes back (the
				// classifier releases the rest) and fail the read, which
				// drops the connection. The bytes stay recorded in the prefix,
				// but the connection is dropped either way. The error must be
				// returned with n==0: io.CopyN and io.ReadFull swallow a
				// trailing error when the requested bytes were all delivered.
				c.budget.Add(-int64(n))
				c.appended -= int64(n)
				return 0, errTotalBudgetExceeded
			}
		}
	}
	return n, err
}

func (c *replayConn) recorded() int { return len(c.prefix) }

// dropLast removes the n most recently recorded bytes from the replay, for
// frames the classifier consumed on behalf of the downstream server, such as
// the ACK of the injected SETTINGS.
func (c *replayConn) dropLast(n int) {
	if n > len(c.prefix) {
		return
	}
	c.prefix = c.prefix[:len(c.prefix)-n]
}

// handoff returns the connection handed over to the server: the recorded
// prefix is replayed first, then the connection continues live.
func (c *replayConn) handoff() *replayConn {
	return &replayConn{Conn: c.Conn, prefix: c.prefix, replay: true}
}

// virtualListener is a net.Listener fed by the classifier. A dedicated
// dispatcher goroutine owns the queue: it forwards connections to Accept and,
// on Close, closes everything still owned, so no connection is ever enqueued
// after shutdown or leaked between delivery and close.
type virtualListener struct {
	addr           net.Addr
	in             chan net.Conn // deliverers; unbuffered so a closed listener never retains anything
	out            chan net.Conn // the server's Accept queue
	done           chan struct{}
	dispatcherDone chan struct{}
	once           sync.Once
}

func newVirtualListener(addr net.Addr) *virtualListener {
	l := &virtualListener{
		addr:           addr,
		in:             make(chan net.Conn),
		out:            make(chan net.Conn, acceptBacklog),
		done:           make(chan struct{}),
		dispatcherDone: make(chan struct{}),
	}
	go l.dispatch()
	return l
}

func (l *virtualListener) dispatch() {
	defer close(l.dispatcherDone)
	for {
		select {
		case c := <-l.in:
			select {
			case l.out <- c:
			case <-l.done:
				_ = c.Close()
				l.drain()
				return
			}
		case <-l.done:
			l.drain()
			return
		}
	}
}

// drain closes every connection still owned by the dispatcher. A connection
// that raced into out and was taken by the server's Accept is not owned
// anymore; a connection still queued is closed exactly once here.
func (l *virtualListener) drain() {
	for {
		select {
		case c := <-l.in:
			_ = c.Close()
		case c := <-l.out:
			_ = c.Close()
		default:
			return
		}
	}
}

func (l *virtualListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.out:
		return c, nil
	case <-l.done:
		// Closed. Everything the dispatcher still owned was closed; a
		// connection that raced out before the close is served by the
		// server's graceful stop, everything else reports closure.
		return nil, net.ErrClosed
	}
}

func (l *virtualListener) Close() error {
	l.once.Do(func() {
		close(l.done)
		<-l.dispatcherDone
	})
	return nil
}

func (l *virtualListener) Addr() net.Addr { return l.addr }

func (l *virtualListener) deliver(c net.Conn) {
	select {
	case l.in <- c:
	case <-l.done:
		_ = c.Close()
	}
}

// isTimeout reports whether the error means a read deadline was exceeded.
func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
