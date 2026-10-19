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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// Header names and content types used across the tests.
const (
	hdrMethod      = ":method"
	hdrContentType = "content-type"
	hdrMethodPost  = "POST"
	ctGRPC         = "application/grpc"
	ctGRPCProto    = "application/grpc+proto"
	ctGRPCWebProto = "application/grpc-web+proto"
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

	_, err := io.WriteString(c, http2.ClientPreface)
	require.NoError(t, err)
	writeRequestFrames(t, c, contentType, extraHeaders...)
}

// encodeRequestHeaders returns the HPACK-encoded header block of a request
// with the given content type.
func encodeRequestHeaders(t *testing.T, contentType string, extraHeaders ...hpack.HeaderField) []byte {
	t.Helper()

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
	return buf.Bytes()
}

// writeRequestFrames sends a client SETTINGS frame and one HEADERS frame
// carrying the given headers: the request flight after the connection
// preface.
func writeRequestFrames(t *testing.T, c net.Conn, contentType string, extraHeaders ...hpack.HeaderField) {
	t.Helper()

	writeFrame(t, c, http2.FrameSettings, 0, 0, nil)
	writeFrame(t, c, http2.FrameHeaders, http2.FlagHeadersEndHeaders, 1,
		encodeRequestHeaders(t, contentType, extraHeaders...))
}

func writeFrame(t *testing.T, c net.Conn, frameType http2.FrameType, flags http2.Flags, streamID uint32, payload []byte) {
	t.Helper()

	hdr := []byte{
		byte(len(payload) >> 16), byte(len(payload) >> 8), byte(len(payload)),
		byte(frameType), byte(flags),
		byte(streamID >> 24), byte(streamID >> 16), byte(streamID >> 8), byte(streamID),
	}
	_, err := c.Write(hdr)
	require.NoError(t, err)
	if len(payload) > 0 {
		_, err = c.Write(payload)
		require.NoError(t, err)
	}
}

// readEOF asserts that the peer closed the connection.
func readEOF(t *testing.T, c net.Conn, within time.Duration) {
	t.Helper()
	require.NoError(t, c.SetReadDeadline(time.Now().Add(within)))
	_, err := c.Read(make([]byte, 1))
	require.Error(t, err)
	assert.False(t, isTimeout(err), "the connection must be closed, not merely silent")
}

// requireAdmissionParked waits until a connection holds an admission slot.
func requireAdmissionParked(t *testing.T, m *Mux, msg string) {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case m.admit <- struct{}{}:
			// The slot was free: put the token back for the matchers.
			<-m.admit
			return false
		default:
			return true
		}
	}, 2*time.Second, 5*time.Millisecond, msg)
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
	// matcher waiting for 24 bytes.
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

			// The gRPC server must see the preface and the request again.
			require.NoError(t, got.SetReadDeadline(time.Now().Add(time.Second)))
			replayed := make([]byte, len(http2.ClientPreface))
			_, err = io.ReadFull(got, replayed)
			require.NoError(t, err)
			assert.Equal(t, http2.ClientPreface, string(replayed), "preface not replayed intact")
		})
	}
}

func TestClassifyGRPCWebGoesToHTTP(t *testing.T) {
	for _, ct := range []string{
		"application/grpc-web",
		ctGRPCWebProto,
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

	_, err = io.WriteString(c, http2.ClientPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2.FrameSettings, 0, 0, nil)
	writeFrame(t, c, http2.FrameWindowUpdate, 0, 0, []byte{0, 0, 0, 1})
	writeFrame(t, c, http2.FrameWindowUpdate, 0, 1, []byte{0, 0, 0, 2})

	var buf bytes.Buffer
	enc := hpack.NewEncoder(&buf)
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrMethod, Value: hdrMethodPost}))
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrContentType, Value: ctGRPCProto}))
	writeFrame(t, c, http2.FrameHeaders, http2.FlagHeadersEndHeaders, 1, buf.Bytes())

	got := acceptWithTimeout(t, m.GRPC())
	got.Close()
}

func TestClassifyPaddedAndPriorityHeaders(t *testing.T) {
	m, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, http2.ClientPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2.FrameSettings, 0, 0, nil)

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
	writeFrame(t, c, http2.FrameHeaders, http2.FlagHeadersPadded|http2.FlagHeadersPriority|http2.FlagHeadersEndHeaders, 1, payload)

	got := acceptWithTimeout(t, m.GRPC())
	got.Close()
}

// TestMatcherDoesNotSpeakFirstForImmediateClients proves the classification
// writes nothing to a connection whose first request arrives without waiting.
// Bytes written while classifying are answered by the client, and the answer
// reaches whichever server ends up owning the connection.
func TestMatcherDoesNotSpeakFirstForImmediateClients(t *testing.T) {
	for _, tc := range []struct {
		name string
		ct   string
		grpc bool
	}{
		{"native gRPC", ctGRPCProto, true},
		{"gRPC-Web", ctGRPCWebProto, false},
		{"plain HTTP/2", "application/json", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// A long grace keeps the matcher silent for immediate requests.
			m, dial := startMux(t, &Options{SettingsGrace: time.Second})

			c, err := dial()
			require.NoError(t, err)
			defer c.Close()

			writeClientStream(t, c, tc.ct)

			// The mux must stay silent while it classifies.
			require.NoError(t, c.SetReadDeadline(time.Now().Add(300*time.Millisecond)))
			_, err = c.Read(make([]byte, 1))
			require.Error(t, err, "the mux must not write during classification")
			assert.True(t, isTimeout(err), "the read must time out, not fail: %v", err)

			if tc.grpc {
				got := acceptWithTimeout(t, m.GRPC())
				got.Close()
			} else {
				got := acceptWithTimeout(t, m.HTTP())
				got.Close()
			}
		})
	}
}

// TestSettingsWaitingClientClassifiesNative covers a client that waits for the
// server's HTTP/2 settings frame before it sends its first request: the gRPC
// Go client works this way. The matcher sends an empty settings frame after
// the grace, the client acknowledges it and sends its request, and the
// connection goes to the gRPC side with the acknowledgement in the replay:
// the gRPC server ignores acknowledgements of settings it never sent.
func TestSettingsWaitingClientClassifiesNative(t *testing.T) {
	m, dial := startMux(t, &Options{SettingsGrace: 100 * time.Millisecond, ClassifyTimeout: 300 * time.Millisecond})

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, http2.ClientPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2.FrameSettings, 0, 0, nil)

	// After the grace, the matcher sends its empty settings frame.
	require.NoError(t, c.SetReadDeadline(time.Now().Add(2*time.Second)))
	injected := make([]byte, 9)
	_, err = io.ReadFull(c, injected)
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 0, 0, byte(http2.FrameSettings), 0, 0, 0, 0, 0}, injected,
		"the matcher must send an empty settings frame to a waiting client")

	// The client acknowledges the injected settings and sends its request:
	// the preface was already sent, only the request flight follows.
	writeFrame(t, c, http2.FrameSettings, http2.FlagSettingsAck, 0, nil)
	writeRequestFrames(t, c, ctGRPCProto)

	got := acceptWithTimeout(t, m.GRPC())
	defer got.Close()

	// The gRPC server sees the whole stream: the preface, the client
	// settings, the acknowledgement of the injected frame, and the request.
	require.NoError(t, got.SetReadDeadline(time.Now().Add(time.Second)))
	replay := make([]byte, len(http2.ClientPreface))
	_, err = io.ReadFull(got, replay)
	require.NoError(t, err)
	assert.Equal(t, http2.ClientPreface, string(replay), "preface not replayed intact")

	// The matcher must clear the write deadline its settings frame was
	// bounded by: cmux clears only the read deadline at handover, and a
	// lingering write deadline would fail every later write on the
	// connection. Write from the server past the classification window and
	// expect the client to receive it.
	time.Sleep(450 * time.Millisecond)
	_, err = got.Write([]byte("still-writable"))
	require.NoError(t, err, "a write deadline survived the handover")
	require.NoError(t, c.SetReadDeadline(time.Now().Add(time.Second)))
	tail := make([]byte, len("still-writable"))
	_, err = io.ReadFull(c, tail)
	require.NoError(t, err)
	assert.Equal(t, "still-writable", string(tail))
}

// TestCommittedNonGRPCDropped proves a spoken-to connection whose first
// request is not native gRPC is closed, not handed to the HTTP server: its
// acknowledgement of the injected settings frame is already recorded, and
// net/http fails a connection that acknowledges settings it never sent.
func TestCommittedNonGRPCDropped(t *testing.T) {
	_, dial := startMux(t, &Options{SettingsGrace: 100 * time.Millisecond, ClassifyTimeout: time.Second})

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, http2.ClientPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2.FrameSettings, 0, 0, nil)

	// Receive the injected settings frame, acknowledge it, and send a
	// gRPC-Web request flight.
	require.NoError(t, c.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = io.ReadFull(c, make([]byte, 9))
	require.NoError(t, err)
	writeFrame(t, c, http2.FrameSettings, http2.FlagSettingsAck, 0, nil)
	writeRequestFrames(t, c, ctGRPCWebProto)

	// The connection matches no listener and cmux closes it.
	readEOF(t, c, 2*time.Second)
}

// TestOversizedHeaderBlockDeclined proves a connection whose first request
// needs more than the classification byte budget goes to the HTTP server,
// whose own header limits dispose of it.
func TestOversizedHeaderBlockDeclined(t *testing.T) {
	m, dial := startMux(t, &Options{MaxClassifiedBytes: 1024})

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	big := strings.Repeat("a", 2048)
	writeClientStream(t, c, ctGRPCProto, hpack.HeaderField{Name: "x-big", Value: big})

	got := acceptWithTimeout(t, m.HTTP())
	got.Close()
}

// TestHugeHPACKStringDeclined proves a single HPACK string beyond the decoder
// bound makes the matcher decline, even when the whole block fits the byte
// budget: a small compressed block must not expand into a huge header value.
func TestHugeHPACKStringDeclined(t *testing.T) {
	m, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, http2.ClientPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2.FrameSettings, 0, 0, nil)

	var buf bytes.Buffer
	enc := hpack.NewEncoder(&buf)
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: hdrContentType, Value: ctGRPCProto}))
	require.NoError(t, enc.WriteField(hpack.HeaderField{Name: "x-big", Value: strings.Repeat("a", 70<<10)}))

	// The block spans the HEADERS frame and CONTINUATION frames: no single
	// frame may exceed the HTTP/2 frame size limit.
	block := buf.Bytes()
	const maxFragment = 1 << 14
	first := block
	if len(first) > maxFragment {
		first = block[:maxFragment]
	}
	writeFrame(t, c, http2.FrameHeaders, 0, 1, first)
	for rest := block[len(first):]; len(rest) > 0; {
		frag := rest
		if len(frag) > maxFragment {
			frag = rest[:maxFragment]
		}
		rest = rest[len(frag):]
		flags := http2.Flags(0)
		if len(rest) == 0 {
			flags = http2.FlagContinuationEndHeaders
		}
		writeFrame(t, c, http2.FrameContinuation, flags, 1, frag)
	}

	got := acceptWithTimeout(t, m.HTTP())
	got.Close()
}

// TestAdmissionControlSheds proves a connection is shed when too many
// connections are in classification: it matches no listener and cmux closes
// it, while the connection that holds the slot stays open.
func TestAdmissionControlSheds(t *testing.T) {
	m, dial := startMux(t, &Options{MaxInflightClassifications: 1, ClassifyTimeout: time.Second})

	// The first connection parks in classification: it sent a preface but no
	// request, so the matcher holds the only slot waiting for frames.
	parked, err := dial()
	require.NoError(t, err)
	defer parked.Close()
	_, err = io.WriteString(parked, http2.ClientPreface)
	require.NoError(t, err)
	requireAdmissionParked(t, m, "the first connection never parked in classification")

	// The second connection prefaced but cannot get a slot in either
	// matcher, so cmux closes it without parsing anything from it.
	shed, err := dial()
	require.NoError(t, err)
	_, err = io.WriteString(shed, http2.ClientPreface)
	require.NoError(t, err)
	readEOF(t, shed, time.Second)

	// The parked connection is still open past its grace: it received the
	// injected settings frame, and no request follows.
	require.NoError(t, parked.SetReadDeadline(time.Now().Add(2*time.Second)))
	injected := make([]byte, 9)
	_, err = io.ReadFull(parked, injected)
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 0, 0, byte(http2.FrameSettings), 0, 0, 0, 0, 0}, injected,
		"the matcher must send an empty settings frame to the parked connection")
	require.NoError(t, parked.SetReadDeadline(time.Now().Add(300*time.Millisecond)))
	_, err = parked.Read(make([]byte, 1))
	require.Error(t, err)
	assert.True(t, isTimeout(err), "the parked connection must stay open: %v", err)
}

// TestIdleConnectionDropped proves a connection that never sends anything is
// closed at the end of the classification window: both matchers decline it.
func TestIdleConnectionDropped(t *testing.T) {
	_, dial := startMux(t, &Options{ClassifyTimeout: 300 * time.Millisecond})

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	readEOF(t, c, 2*time.Second)
}

// TestStalledAfterPrefaceDropped proves a connection that sends a preface and
// then stalls is closed at the end of its classification window: the matcher
// speaks to it after the grace, and a spoken-to connection never reaches the
// HTTP server.
func TestStalledAfterPrefaceDropped(t *testing.T) {
	_, dial := startMux(t, &Options{SettingsGrace: 100 * time.Millisecond, ClassifyTimeout: 300 * time.Millisecond})

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, http2.ClientPreface)
	require.NoError(t, err)

	// The matcher speaks after the grace; the connection never sends its
	// request and is dropped at the classification deadline.
	require.NoError(t, c.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = io.ReadFull(c, make([]byte, 9))
	require.NoError(t, err)
	readEOF(t, c, 2*time.Second)
}

// TestSlowMidFrameClientTakesFallback proves a client whose first request
// straddles the settings grace is served, not dropped: the grace fires
// mid-frame, the matcher declines without speaking, and the intact replay
// lets the HTTP server parse the completed request through the documented
// fallback (D-014). The framer cannot resume a half-read frame, so speaking
// there would desynchronize it.
func TestSlowMidFrameClientTakesFallback(t *testing.T) {
	m, dial := startMux(t, &Options{SettingsGrace: 150 * time.Millisecond, ClassifyTimeout: 2 * time.Second})

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	_, err = io.WriteString(c, http2.ClientPreface)
	require.NoError(t, err)
	writeFrame(t, c, http2.FrameSettings, 0, 0, nil)

	// Send the HEADERS frame header and half of its payload, then stall past
	// the grace.
	block := encodeRequestHeaders(t, ctGRPCProto)
	hdr := []byte{
		byte(len(block) >> 16), byte(len(block) >> 8), byte(len(block)),
		byte(http2.FrameHeaders), byte(http2.FlagHeadersEndHeaders),
		0, 0, 0, 1,
	}
	_, err = c.Write(hdr)
	require.NoError(t, err)
	_, err = c.Write(block[:len(block)/2])
	require.NoError(t, err)
	time.Sleep(400 * time.Millisecond)

	// The rest of the request arrives after the grace.
	_, err = c.Write(block[len(block)/2:])
	require.NoError(t, err)

	// The connection is not dropped and not poisoned: the HTTP server gets
	// it with the whole stream replayed.
	got := acceptWithTimeout(t, m.HTTP())
	defer got.Close()
	require.NoError(t, got.SetReadDeadline(time.Now().Add(time.Second)))
	replay := make([]byte, len(http2.ClientPreface))
	_, err = io.ReadFull(got, replay)
	require.NoError(t, err)
	assert.Equal(t, http2.ClientPreface, string(replay), "preface not replayed intact")
}

func TestCloseClosesQueuedConnections(t *testing.T) {
	m, dial := startMux(t, &Options{ClassifyTimeout: 300 * time.Millisecond})

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()

	writeClientStream(t, c, ctGRPCProto)

	// Leave the classified connection queued: no server accepts it.
	// Classification completes in microseconds locally; 100ms is a wide
	// margin.
	time.Sleep(100 * time.Millisecond)
	m.Close()

	readEOF(t, c, 2*time.Second)
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

	m.Close()

	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			require.True(t, m.IsOrderlyShutdownErr(err), "unexpected accept error: %v", err)
		case <-time.After(2 * time.Second):
			require.Fail(t, "Accept was not unblocked by Close")
		}
	}
}

// TestSubListenerCloseMarksStopping proves the shutdown race tolerance from
// the other side: a server that stops first closes the root listener through
// its sub-listener, and the mux reports an orderly shutdown so the remaining
// server can tell it apart from a fatal accept failure.
func TestSubListenerCloseMarksStopping(t *testing.T) {
	m, dial := startMux(t, nil)

	c, err := dial()
	require.NoError(t, err)
	defer c.Close()
	writeClientStream(t, c, ctGRPCProto)

	require.NoError(t, m.GRPC().Close())
	assert.True(t, m.Stopping(), "closing a sub-listener must mark the mux as stopping")

	errCh := make(chan error, 1)
	go func() {
		_, err := m.HTTP().Accept()
		errCh <- err
	}()
	select {
	case err := <-errCh:
		require.True(t, m.IsOrderlyShutdownErr(err), "unexpected accept error: %v", err)
	case <-time.After(2 * time.Second):
		require.Fail(t, "the HTTP listener did not close with the mux")
	}
}

// TestCloseReturnsWhileConnParked proves Close does not ride out an idle
// classification beyond the classification window.
func TestCloseReturnsWhileConnParked(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	m := New(l, &Options{ClassifyTimeout: 300 * time.Millisecond, MaxInflightClassifications: 1})
	require.NoError(t, m.Start(context.Background()))

	c, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)
	defer c.Close()

	// A connection that produced a byte parks in classification while it
	// holds the admission slot: exactly the state Close must not ride out. A
	// silent connection holds no slot at all.
	_, err = io.WriteString(c, http2.ClientPreface)
	require.NoError(t, err)
	requireAdmissionParked(t, m, "the connection never parked in classification")
	start := time.Now()
	m.Close()
	require.Less(t, time.Since(start), 2*time.Second, "Close rode out the classification window")
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

	// ALPN http/1.1 -> HTTP server.
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

// TestGRPCServeOverMux runs a real grpc-go server and client through the mux,
// over plaintext and over TLS: the exact wiring the control plane uses. It
// guards the handoff of matched connections to grpc.Server.Serve.
func TestGRPCServeOverMux(t *testing.T) {
	for _, tc := range []struct {
		name string
		tls  bool
	}{
		{"plaintext", false},
		{"TLS", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)

			root := l
			if tc.tls {
				root = tls.NewListener(l, &tls.Config{
					Certificates: []tls.Certificate{selfSignedCert(t)},
					MinVersion:   tls.VersionTLS12,
					NextProtos:   []string{"h2", "http/1.1"},
				})
			}
			m := New(root, nil)
			require.NoError(t, m.Start(context.Background()))
			t.Cleanup(m.Close)

			srv := grpc.NewServer()
			grpc_health_v1.RegisterHealthServer(srv, health.NewServer())
			go func() { _ = srv.Serve(m.GRPC()) }()
			t.Cleanup(srv.Stop)

			creds := insecure.NewCredentials()
			if tc.tls {
				creds = credentials.NewTLS(&tls.Config{
					InsecureSkipVerify: true, //nolint:gosec // test server with a self-signed certificate
					MinVersion:         tls.VersionTLS12,
				})
			}
			conn, err := grpc.NewClient(l.Addr().String(), grpc.WithTransportCredentials(creds))
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			resp, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
			require.NoError(t, err)
			assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.GetStatus())
		})
	}
}

func TestIsNativeGRPCContentType(t *testing.T) {
	for _, tc := range []struct {
		ct   string
		want bool
	}{
		{"application/grpc", true},
		{"application/grpc+proto", true},
		{"application/grpc+json", true},
		{"application/grpc; charset=utf-8", true},
		{"Application/GRPC", true},
		{"application/grpc-web", false},
		{"application/grpc-web+proto", false},
		{"application/grpc-web+json", false},
		{"application/json", false},
		{"text/plain", false},
		{"", false},
	} {
		t.Run(tc.ct, func(t *testing.T) {
			assert.Equal(t, tc.want, IsNativeGRPCContentType(tc.ct))
		})
	}
}
