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

package httpgrpcmux

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2/hpack"
)

// Header names and content types used across the tests.
const (
	hdrMethod      = ":method"
	hdrContentType = "content-type"
	hdrMethodPost  = "POST"
	ctGRPC         = "application/grpc"
	ctGRPCProto    = "application/grpc+proto"
)

// startMux creates a real TCP listener, wraps it in a mux and starts it.
// It returns the mux and the client-side dial function.
func startMux(t *testing.T, opts *Options) (*Mux, func() (net.Conn, error)) {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })

	m := New(l, opts)
	require.NoError(t, m.Start(context.Background()))
	t.Cleanup(func() { m.Close() })

	dial := func() (net.Conn, error) { return net.Dial("tcp", l.Addr().String()) }
	return m, dial
}

// acceptWithTimeout waits for a connection on the given listener.
func acceptWithTimeout(t *testing.T, l net.Listener) net.Conn {
	t.Helper()
	type result struct {
		c   net.Conn
		err error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := l.Accept()
		ch <- result{c, err}
	}()
	select {
	case r := <-ch:
		require.NoError(t, r.err)
		return r.c
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for the connection to be classified")
		return nil
	}
}

// writeClientStream sends an HTTP/2 client preface, a client SETTINGS frame
// and one HEADERS frame carrying the given headers.
func writeClientStream(t *testing.T, c net.Conn, contentType string, extraHeaders ...hpack.HeaderField) {
	t.Helper()

	_, err := io.WriteString(c, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2FrameSettings, 0, 0, nil)

	var buf bytes.Buffer
	enc := hpack.NewEncoder(&buf)
	for _, hf := range append([]hpack.HeaderField{
		{Name: hdrMethod, Value: hdrMethodPost},
		{Name: ":scheme", Value: "http"},
		{Name: ":path", Value: "/controlplane.v1.FooService/Bar"},
		{Name: hdrContentType, Value: contentType},
	}, extraHeaders...) {
		require.NoError(t, enc.WriteField(hf))
	}
	writeFrame(t, c, http2FrameHeaders, http2FlagEndHeaders, 1, buf.Bytes())
}

func writeFrame(t *testing.T, c net.Conn, frameType, flags byte, streamID uint32, payload []byte) {
	t.Helper()

	hdr := []byte{
		byte(len(payload) >> 16), byte(len(payload) >> 8), byte(len(payload)),
		frameType, flags,
		byte(streamID >> 24), byte(streamID >> 16), byte(streamID >> 8), byte(streamID),
	}
	_, err := c.Write(hdr)
	require.NoError(t, err)
	if len(payload) > 0 {
		_, err = c.Write(payload)
		require.NoError(t, err)
	}
}

func TestClassifyHTTP1(t *testing.T) {
	m, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	request := "GET /statusz HTTP/1.1\r\nHost: example.com\r\n\r\n"
	_, err = io.WriteString(c, request)
	require.NoError(t, err)

	got := acceptWithTimeout(t, m.HTTP())
	defer got.Close()

	require.NoError(t, got.SetReadDeadline(time.Now().Add(time.Second)))
	data := make([]byte, len(request))
	_, err = io.ReadFull(got, data)
	require.NoError(t, err)
	assert.Equal(t, request, string(data), "the HTTP server must receive the request intact")
}

func TestClassifyShortHTTP10Request(t *testing.T) {
	// A request shorter than the HTTP/2 preface must not deadlock the
	// classifier waiting for 24 bytes.
	m, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	request := "GET / HTTP/1.0\r\n\r\n" // 18 bytes
	_, err = io.WriteString(c, request)
	require.NoError(t, err)

	got := acceptWithTimeout(t, m.HTTP())
	defer got.Close()

	require.NoError(t, got.SetReadDeadline(time.Now().Add(time.Second)))
	data := make([]byte, len(request))
	_, err = io.ReadFull(got, data)
	require.NoError(t, err)
	assert.Equal(t, request, string(data), "short request not replayed intact")
}

func TestClassifyNativeGRPC(t *testing.T) {
	for _, ct := range []string{
		ctGRPC,
		ctGRPCProto,
		"application/grpc+json",
		ctGRPC + "; charset=utf-8", // content type with parameters
	} {
		t.Run(ct, func(t *testing.T) {
			m, dial := startMux(t, nil)

			c, err := dial()
			require.NoError(t, err)
			defer c.Close()

			writeClientStream(t, c, ct)

			got := acceptWithTimeout(t, m.GRPC())
			defer got.Close()

			// The gRPC server must see the preface again.
			require.NoError(t, got.SetReadDeadline(time.Now().Add(time.Second)))
			prefix := make([]byte, len(clientConnPreface))
			_, err = io.ReadFull(got, prefix)
			require.NoError(t, err)
			assert.Equal(t, clientConnPreface, string(prefix), "preface not replayed intact")
		})
	}
}

func TestClassifyGRPCWebGoesToHTTP(t *testing.T) {
	for _, ct := range []string{
		"application/grpc-web",
		"application/grpc-web+proto",
		"application/grpc-web+json",
	} {
		t.Run(ct, func(t *testing.T) {
			m, dial := startMux(t, nil)

			c, err := dial()
			require.NoError(t, err)
			defer c.Close()

			writeClientStream(t, c, ct)

			got := acceptWithTimeout(t, m.HTTP())
			got.Close()
		})
	}
}

func TestClassifyPlainH2GoesToHTTP(t *testing.T) {
	m, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	writeClientStream(t, c, "application/json")

	got := acceptWithTimeout(t, m.HTTP())
	got.Close()
}

func TestClassifySkipsIntermediateFrames(t *testing.T) {
	// WINDOW_UPDATE frames between the preface and the first HEADERS must
	// not disturb the classification.
	m, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2FrameSettings, 0, 0, nil)
	writeFrame(t, c, 0x8 /* WINDOW_UPDATE */, 0, 0, []byte{0, 0, 0, 1})
	writeFrame(t, c, 0x8 /* WINDOW_UPDATE */, 0, 1, []byte{0, 0, 0, 2})

	var buf bytes.Buffer
	enc := hpack.NewEncoder(&buf)
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrMethod, Value: hdrMethodPost}))
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrContentType, Value: ctGRPCProto}))
	writeFrame(t, c, http2FrameHeaders, http2FlagEndHeaders, 1, buf.Bytes())

	got := acceptWithTimeout(t, m.GRPC())
	got.Close()
}

func TestClassifyPaddedAndPriorityHeaders(t *testing.T) {
	m, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2FrameSettings, 0, 0, nil)

	var buf bytes.Buffer
	enc := hpack.NewEncoder(&buf)
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrMethod, Value: hdrMethodPost}))
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrContentType, Value: ctGRPCProto}))

	// PADDED (0x8) + PRIORITY (0x20) + END_HEADERS (0x4)
	// Payload: [pad length][stream dependency][weight][fragment][padding]
	frag := buf.Bytes()
	payload := make([]byte, 0, 6+len(frag))
	payload = append(payload, 2)              // pad length = 2
	payload = append(payload, 0, 0, 0, 0, 16) // stream dep + weight
	payload = append(payload, frag...)        // header block fragment
	payload = append(payload, 0, 0)           // padding
	writeFrame(t, c, http2FrameHeaders, 0x8|0x20|0x4, 1, payload)

	got := acceptWithTimeout(t, m.GRPC())
	got.Close()
}

// TestSettingsAckNotReplayed proves that the ACK of the injected SETTINGS is
// removed from the replay: the downstream gRPC server must never see it, or it
// would count two SETTINGS acknowledgements and fail with PROTOCOL_ERROR.
func TestSettingsAckNotReplayed(t *testing.T) {
	m, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2FrameSettings, 0, 0, nil)

	// The classifier injects its empty SETTINGS after the grace period.
	require.NoError(t, c.SetReadDeadline(time.Now().Add(5*time.Second)))
	hdr := make([]byte, 9)
	_, err = io.ReadFull(c, hdr)
	require.NoError(t, err)

	// The client acknowledges the injected SETTINGS, then sends its request.
	writeFrame(t, c, http2FrameSettings, http2FlagSettingsAck, 0, nil)
	writeClientStreamAfterPreface(t, c)

	got := acceptWithTimeout(t, m.GRPC())
	defer got.Close()

	// Parse the frames the server receives: SETTINGS (client) then HEADERS,
	// and no SETTINGS-ACK.
	require.NoError(t, got.SetReadDeadline(time.Now().Add(time.Second)))
	reader := bufio.NewReader(got)
	preface := make([]byte, len(clientConnPreface))
	_, err = io.ReadFull(reader, preface)
	require.NoError(t, err)
	assert.Equal(t, clientConnPreface, string(preface))

	type frame struct {
		frameType byte
		flags     byte
	}
	var frames []frame
	for {
		var fh [9]byte
		if _, err := io.ReadFull(reader, fh[:]); err != nil {
			break // idle connection: everything available was parsed
		}
		length := int(fh[0])<<16 | int(fh[1])<<8 | int(fh[2])
		if length > 0 {
			if _, err := io.CopyN(io.Discard, reader, int64(length)); err != nil {
				break
			}
		}
		frames = append(frames, frame{fh[3], fh[4]})
	}

	require.NotEmpty(t, frames)
	assert.Equal(t, byte(http2FrameSettings), frames[0].frameType, "first replayed frame must be the client SETTINGS")
	assert.Equal(t, byte(0), frames[0].flags&http2FlagSettingsAck, "first replayed frame must not be an ACK")
	foundAck := false
	for _, f := range frames {
		if f.frameType == http2FrameSettings && f.flags&http2FlagSettingsAck != 0 {
			foundAck = true
		}
	}
	assert.False(t, foundAck, "the ACK of the injected SETTINGS must not reach the server")
}

// TestLateSettingsAckNotReplayed proves that the ACK of the injected SETTINGS
// is removed from the replay even when it arrives after the first request: a
// client whose frame-read loop races its request-write goroutine can send
// HEADERS first and the ACK afterwards. The classifier keeps consuming frames
// until the ACK shows up, drops it, and hands the connection to the HTTP
// server without it.
func TestLateSettingsAckNotReplayed(t *testing.T) {
	m, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2FrameSettings, 0, 0, nil)

	// The classifier injects its empty SETTINGS after the grace period.
	require.NoError(t, c.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = io.ReadFull(c, make([]byte, 9))
	require.NoError(t, err)

	// The client sends its (non-gRPC) request BEFORE acknowledging the
	// injected SETTINGS, and the ACK only afterwards.
	var buf bytes.Buffer
	enc := hpack.NewEncoder(&buf)
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrMethod, Value: hdrMethodPost}))
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrContentType, Value: "application/json"}))
	writeFrame(t, c, http2FrameHeaders, http2FlagEndHeaders, 1, buf.Bytes())
	writeFrame(t, c, http2FrameSettings, http2FlagSettingsAck, 0, nil)

	// The connection must still reach the HTTP listener: the late ACK does
	// not change the classification, it only delays the handover.
	got := acceptWithTimeout(t, m.HTTP())
	defer got.Close()

	// Parse the frames the server receives: SETTINGS (client) then HEADERS,
	// and no SETTINGS-ACK.
	require.NoError(t, got.SetReadDeadline(time.Now().Add(time.Second)))
	reader := bufio.NewReader(got)
	preface := make([]byte, len(clientConnPreface))
	_, err = io.ReadFull(reader, preface)
	require.NoError(t, err)
	assert.Equal(t, clientConnPreface, string(preface))

	type frame struct {
		frameType byte
		flags     byte
	}
	var frames []frame
	for {
		var fh [9]byte
		if _, err := io.ReadFull(reader, fh[:]); err != nil {
			break // idle connection: everything available was parsed
		}
		length := int(fh[0])<<16 | int(fh[1])<<8 | int(fh[2])
		if length > 0 {
			if _, err := io.CopyN(io.Discard, reader, int64(length)); err != nil {
				break
			}
		}
		frames = append(frames, frame{fh[3], fh[4]})
	}

	require.NotEmpty(t, frames)
	assert.Equal(t, byte(http2FrameSettings), frames[0].frameType, "first replayed frame must be the client SETTINGS")
	foundAck := false
	sawHeaders := false
	for _, f := range frames {
		if f.frameType == http2FrameSettings && f.flags&http2FlagSettingsAck != 0 {
			foundAck = true
		}
		if f.frameType == http2FrameHeaders {
			sawHeaders = true
		}
	}
	assert.True(t, sawHeaders, "the client's HEADERS must reach the server")
	assert.False(t, foundAck, "the ACK of the injected SETTINGS must not reach the server")
}

// TestLateAckWaitDropsConnection proves the bound on the wait for a late
// SETTINGS ACK: a client that sends its request but never acknowledges the
// injected SETTINGS gets the connection dropped shortly after — the HTTP/2
// connection would break on the extra ACK anyway — and never handed to a
// server.
func TestLateAckWaitDropsConnection(t *testing.T) {
	// A short grace keeps the whole classification far from the end of the
	// default 10s classification window, so the drop proves the dedicated
	// ACK-wait bound, not the classification timeout.
	m, dial := startMux(t, &Options{SettingsGrace: 100 * time.Millisecond})

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2FrameSettings, 0, 0, nil)

	// Consume the injected SETTINGS the classifier sends after the grace.
	require.NoError(t, c.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = io.ReadFull(c, make([]byte, 9))
	require.NoError(t, err)

	// The client sends a non-gRPC request and never ACKs.
	var buf bytes.Buffer
	enc := hpack.NewEncoder(&buf)
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrMethod, Value: hdrMethodPost}))
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrContentType, Value: "application/json"}))
	writeFrame(t, c, http2FrameHeaders, http2FlagEndHeaders, 1, buf.Bytes())

	// The classifier must close the connection when the wait for the ACK
	// expires (bounded at 2s), well before the classification window (10s)
	// would. The client's own 5s read deadline proves it: a connection held
	// past it would surface here as a deadline error instead.
	require.NoError(t, c.SetReadDeadline(time.Now().Add(5*time.Second)))
	start := time.Now()
	_, err = c.Read(make([]byte, 9))
	require.Error(t, err, "expected the connection to be dropped when the ACK never arrives")
	require.NotErrorIs(t, err, os.ErrDeadlineExceeded, "the connection was closed by the classifier, not by the client's deadline")
	require.Less(t, time.Since(start), 5*time.Second, "the classifier held the connection past the ACK wait bound")

	// Nothing must have been delivered to either server.
	time.Sleep(200 * time.Millisecond)
	type result struct {
		c   net.Conn
		err error
	}
	for _, l := range []net.Listener{m.GRPC(), m.HTTP()} {
		ch := make(chan result, 1)
		go func(l net.Listener) {
			c, err := l.Accept()
			ch <- result{c, err}
		}(l)
		select {
		case r := <-ch:
			if r.err == nil {
				r.c.Close()
				assert.Fail(t, "the connection was delivered to a server despite the missing ACK")
			}
		case <-time.After(100 * time.Millisecond):
			// Nothing delivered: correct.
		}
	}
}

func TestOversizedFrameDropped(t *testing.T) {
	// A frame payload that would exceed the classification budget must get
	// the connection dropped, not buffered.
	_, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2FrameSettings, 0, 0, nil)

	// A WINDOW_UPDATE whose length field claims almost 16 MiB: the classifier
	// must reject it instead of recording it.
	huge := []byte{0xFF, 0xFF, 0xFF, 0x8, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err = c.Write(huge)
	require.NoError(t, err)

	require.NoError(t, c.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = c.Read(make([]byte, 9))
	require.Error(t, err, "expected the connection to be dropped for the oversized frame")
}

func TestCloseClosesQueuedConnections(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	m := New(l, nil)

	// Queue a classified connection without any server accepting it.
	vl := m.GRPC().(*virtualListener)
	serverEnd, clientEnd := net.Pipe()
	vl.deliver(serverEnd)
	defer clientEnd.Close()

	m.Close()

	// The queued connection must have been closed by Close, not leaked.
	// SetReadDeadline may already fail because the pipe end was closed,
	// which is exactly what we are proving; the Read below distinguishes
	// closure from a timeout.
	_ = clientEnd.SetReadDeadline(time.Now().Add(time.Second))
	_, err = clientEnd.Read(make([]byte, 1))
	require.Error(t, err)
	// A read timeout would mean the connection is still open: Close must
	// actually have closed it.
	require.NotErrorIs(t, err, os.ErrDeadlineExceeded, "the queued connection was not closed when the mux stopped")
}

// TestAggregateBudgetShedsConnections proves the aggregate byte budget: once
// the total recorded bytes across in-flight classifications exceed it, new
// recording fails and the connection is dropped.
func TestAggregateBudgetShedsConnections(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	m := New(l, &Options{MaxTotalClassifiedBytes: 4096, ClassifyTimeout: 5 * time.Second})
	require.NoError(t, m.Start(context.Background()))
	t.Cleanup(m.Close)

	dial := func() (net.Conn, error) { return net.Dial("tcp", l.Addr().String()) }

	// First connection records ~3 KiB and stays in classification.
	c1, err := dial()
	require.NoError(t, err)
	defer c1.Close()
	_, err = io.WriteString(c1, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c1, http2FrameSettings, 0, 0, nil)
	writeFrame(t, c1, 0x8 /* WINDOW_UPDATE */, 0, 0, bytes.Repeat([]byte{0}, 3072))
	// Wait until the first connection's bytes are all accounted for — exactly:
	// nothing else has been sent, so this state is stable — so the second
	// connection deterministically trips the aggregate budget.
	require.Eventually(t, func() bool { return m.budget.Load() == 3114 },
		2*time.Second, 10*time.Millisecond, "first connection's bytes were not recorded")

	// Second connection pushes the aggregate over the budget: its recording
	// must fail and its connection be dropped.
	c2, err := dial()
	require.NoError(t, err)
	defer c2.Close()
	_, err = io.WriteString(c2, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c2, http2FrameSettings, 0, 0, nil)
	writeFrame(t, c2, 0x8 /* WINDOW_UPDATE */, 0, 0, bytes.Repeat([]byte{0}, 2048))

	require.NoError(t, c2.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = c2.Read(make([]byte, 9))
	require.Error(t, err, "expected the connection to be dropped when the aggregate budget is exhausted")
	require.NotErrorIs(t, err, os.ErrDeadlineExceeded, "connection was not dropped; the read merely timed out")
}

// TestPayloadAllocationChargedToBudget proves that a frame payload's
// allocation is charged against the aggregate budget before it happens: a
// connection that declares a large payload while the aggregate budget is
// nearly exhausted must be dropped instead of silently allocating the bytes
// and contributing nothing to the budget. The refused reservation must also
// be released, keeping the accounting exact.
func TestPayloadAllocationChargedToBudget(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	m := New(l, &Options{MaxTotalClassifiedBytes: 4096, ClassifyTimeout: 5 * time.Second})
	require.NoError(t, m.Start(context.Background()))
	t.Cleanup(m.Close)

	dial := func() (net.Conn, error) { return net.Dial("tcp", l.Addr().String()) }

	// First connection records ~3 KiB and stays in classification.
	c1, err := dial()
	require.NoError(t, err)
	defer c1.Close()
	_, err = io.WriteString(c1, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c1, http2FrameSettings, 0, 0, nil)
	writeFrame(t, c1, 0x8 /* WINDOW_UPDATE */, 0, 0, bytes.Repeat([]byte{0}, 3072))
	require.Eventually(t, func() bool { return m.budget.Load() == 3114 },
		2*time.Second, 10*time.Millisecond, "first connection's bytes were not recorded")

	// Second connection declares a 2 KiB HEADERS payload without sending it:
	// its recorded bytes (3156) plus the 2048 reservation exceed the 4096
	// budget, so the allocation must be refused and the connection dropped at
	// once, instead of allocating the bytes and only noticing when the
	// payload trickles in.
	c2, err := dial()
	require.NoError(t, err)
	defer c2.Close()
	_, err = io.WriteString(c2, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c2, http2FrameSettings, 0, 0, nil)
	// HEADERS frame header claiming 2048 bytes of payload, never sent.
	_, err = c2.Write([]byte{0x00, 0x08, 0x00, http2FrameHeaders, 0x00, 0, 0, 0, 1})
	require.NoError(t, err)

	require.NoError(t, c2.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = c2.Read(make([]byte, 9))
	require.Error(t, err, "expected the connection to be dropped when the payload allocation exceeds the budget")
	require.NotErrorIs(t, err, os.ErrDeadlineExceeded, "connection was not dropped; the read merely timed out")

	// The refused reservation must leave the accounting exact: only the first
	// connection's share stays charged.
	require.Eventually(t, func() bool { return m.budget.Load() == 3114 },
		2*time.Second, 10*time.Millisecond, "the refused reservation was not released")
}

// TestHPACKAmplificationRejected proves the decoded-header-list caps: a tiny
// compressed block that expands into thousands of header fields must get the
// connection dropped instead of being amplified in memory.
func TestHPACKAmplificationRejected(t *testing.T) {
	_, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2FrameSettings, 0, 0, nil)

	var buf bytes.Buffer
	enc := hpack.NewEncoder(&buf)
	big := hpack.HeaderField{Name: "x-bomb", Value: strings.Repeat("a", 1024)}
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrMethod, Value: hdrMethodPost}))
	require.NoError(t, enc.WriteField(big))
	// One-byte indexed references to the big field: the compressed block stays
	// small while the decoded list grows past the cap.
	for i := 0; i < 2000; i++ {
		require.NoError(t, enc.WriteField(big))
	}
	writeFrame(t, c, http2FrameHeaders, http2FlagEndHeaders, 1, buf.Bytes())

	require.NoError(t, c.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = c.Read(make([]byte, 9))
	require.Error(t, err, "expected the connection to be dropped for the amplified header list")
}

// TestAckWithoutInjectionDropped proves that a SETTINGS-ACK for which the
// classifier never injected any SETTINGS is rejected: no server SETTINGS were
// sent before the handover, so such an ACK is malformed traffic.
func TestAckWithoutInjectionDropped(t *testing.T) {
	_, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2FrameSettings, 0, 0, nil)
	writeFrame(t, c, http2FrameSettings, http2FlagSettingsAck, 0, nil)

	require.NoError(t, c.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = c.Read(make([]byte, 9))
	require.Error(t, err, "expected the connection to be dropped for the unexpected ACK")
}

// TestMalformedAckDropped proves that a SETTINGS-ACK with a payload is
// rejected: replaying its bytes as frame headers would rewrite the client's
// frame stream.
func TestMalformedAckDropped(t *testing.T) {
	_, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2FrameSettings, 0, 0, nil)

	// Trigger the injected SETTINGS, acknowledge it with a bogus payload.
	require.NoError(t, c.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = io.ReadFull(c, make([]byte, 9))
	require.NoError(t, err)
	writeFrame(t, c, http2FrameSettings, http2FlagSettingsAck, 0, []byte{0, 0, 0, 1})

	require.NoError(t, c.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = c.Read(make([]byte, 9))
	require.Error(t, err, "expected the connection to be dropped for the malformed ACK")
}

// eofAtEndConn returns the whole payload together with a trailing io.EOF, as
// a TLS connection may when the peer half-closes right after its last record.
type eofAtEndConn struct {
	net.Conn
	data []byte
	sent bool
}

func (c *eofAtEndConn) Read(p []byte) (int, error) {
	if c.sent {
		return 0, io.EOF
	}
	c.sent = true
	return copy(p, c.data), io.EOF
}

// TestReadIntoToleratesTrailingEOF proves readInto honors the io.Reader
// contract: a Read that returns the completing bytes together with an error
// must not fail the read.
func TestReadIntoToleratesTrailingEOF(t *testing.T) {
	bc := &replayConn{Conn: &eofAtEndConn{data: []byte("0123456789")}, recording: true}
	injected := false
	buf := make([]byte, 10)
	require.NoError(t, readInto(bc, buf, &injected, time.Now().Add(time.Second)))
	assert.Equal(t, "0123456789", string(buf))
}

func TestServerSpeaksFirst(t *testing.T) {
	// A gRPC client that waits for the server's SETTINGS before sending its
	// first request must still be classified.
	m, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, clientConnPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2FrameSettings, 0, 0, nil)

	// The classifier must send its empty SETTINGS frame after the grace
	// period, unblocking the waiting client.
	require.NoError(t, c.SetReadDeadline(time.Now().Add(5*time.Second)))
	hdr := make([]byte, 9)
	_, err = io.ReadFull(c, hdr)
	require.NoError(t, err)
	assert.Equal(t, byte(http2FrameSettings), hdr[3], "expected a SETTINGS frame from the classifier")
	assert.Zero(t, hdr[4]&http2FlagSettingsAck, "expected a SETTINGS frame, not an ACK")

	writeClientStreamAfterPreface(t, c)

	got := acceptWithTimeout(t, m.GRPC())
	got.Close()
}

// writeClientStreamAfterPreface sends SETTINGS + HEADERS on a connection that
// already sent its preface.
func writeClientStreamAfterPreface(t *testing.T, c net.Conn) {
	t.Helper()

	writeFrame(t, c, http2FrameSettings, 0, 0, nil)
	var buf bytes.Buffer
	enc := hpack.NewEncoder(&buf)
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrMethod, Value: hdrMethodPost}))
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrContentType, Value: ctGRPCProto}))
	writeFrame(t, c, http2FrameHeaders, http2FlagEndHeaders, 1, buf.Bytes())
}

func TestIdleConnectionDropped(t *testing.T) {
	m, dial := startMux(t, &Options{ClassifyTimeout: 100 * time.Millisecond})

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	// The client sends nothing: the classifier must give up on the idle
	// connection after the deadline, not hold it.
	require.NoError(t, c.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = c.Read(make([]byte, 9))
	require.Error(t, err, "expected the connection to be closed after the classification timeout")

	// Give the mux a moment to drop it, then make sure nothing was delivered.
	time.Sleep(200 * time.Millisecond)
	type result struct {
		c   net.Conn
		err error
	}
	for _, l := range []net.Listener{m.GRPC(), m.HTTP()} {
		ch := make(chan result, 1)
		go func(l net.Listener) {
			c, err := l.Accept()
			ch <- result{c, err}
		}(l)
		select {
		case r := <-ch:
			if r.err == nil {
				r.c.Close()
				assert.Fail(t, "idle connection was delivered to a server")
			}
		case <-time.After(100 * time.Millisecond):
			// Nothing delivered: correct.
		}
	}
}

func TestCloseUnblocksAccept(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	m := New(l, nil)
	require.NoError(t, m.Start(context.Background()))

	errCh := make(chan error, 2)
	for _, lis := range []net.Listener{m.GRPC(), m.HTTP()} {
		go func(lis net.Listener) {
			_, err := lis.Accept()
			errCh <- err
		}(lis)
	}

	time.Sleep(50 * time.Millisecond)
	m.Close()

	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			require.ErrorIs(t, err, net.ErrClosed)
		case <-time.After(2 * time.Second):
			require.Fail(t, "Accept was not unblocked by Close")
		}
	}
}

func TestTLSALPNClassification(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{selfSignedCert(t)},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"h2", "http/1.1"},
	}
	m := New(tls.NewListener(l, tlsConfig), nil)
	require.NoError(t, m.Start(context.Background()))
	t.Cleanup(m.Close)

	dialTLS := func(t *testing.T, alpn string) net.Conn {
		t.Helper()
		conn, err := tls.Dial("tcp", l.Addr().String(), &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // test server with a self-signed certificate
			NextProtos:         []string{alpn},
			MinVersion:         tls.VersionTLS12,
		})
		require.NoError(t, err)
		return conn
	}

	// ALPN h2 with a gRPC first request -> gRPC server.
	c := dialTLS(t, "h2")
	writeClientStream(t, c, ctGRPCProto)
	got := acceptWithTimeout(t, m.GRPC())
	got.Close()
	c.Close()

	// ALPN http/1.1 -> HTTP server without any frame parsing.
	c = dialTLS(t, "http/1.1")
	_, err = io.WriteString(c, "GET /statusz HTTP/1.1\r\nHost: x\r\n\r\n")
	require.NoError(t, err)
	got = acceptWithTimeout(t, m.HTTP())
	got.Close()
	c.Close()
}

func selfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// TestCloseCoversConnectionAcceptedBeforeClassification proves the tracking
// has no window: a connection accepted but not yet inside its classify
// goroutine is still closed by Close, so shutdown does not ride out the
// classification deadline.
func TestCloseCoversConnectionAcceptedBeforeClassification(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	m := New(l, nil)
	require.NoError(t, m.Start(context.Background()))

	c, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)
	defer c.Close()

	// Wait until a connection is registered in the inflight set (keyed by the
	// server-side connection, not the client side), which is what Close walks
	// — this is exactly the state the race used to miss.
	require.Eventually(t, func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return len(m.inflight) > 0
	}, 2*time.Second, 5*time.Millisecond, "the accepted connection was not registered in the inflight set")

	start := time.Now()
	m.Close()
	// The idle connection's deadline (10s) must not gate the shutdown.
	require.Less(t, time.Since(start), 2*time.Second)
}
