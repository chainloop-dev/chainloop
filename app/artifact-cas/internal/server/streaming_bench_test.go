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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/bytestream"
)

// This compares the production CAS service and interceptors over the unchanged
// legacy listener and the shared cmux endpoint, both on the native transport
// with the same disk-backed test backend.
// It is a transport experiment, not the original R-003 release/S3 qualification.
// Use a disk-backed TMPDIR: /tmp may be tmpfs on development machines.
func BenchmarkCASByteStreamTransport(b *testing.B) {
	size := int64(1 << 30)
	if value := os.Getenv("CAS_BENCH_BYTES"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		require.NoError(b, err)
		require.Positive(b, parsed)
		size = parsed
	}
	hash := sha256.New()
	_, err := io.CopyN(hash, zeroReader{}, size)
	require.NoError(b, err)
	digest := hex.EncodeToString(hash.Sum(nil))
	for _, endpoint := range []string{casLegacyEndpoint, casSharedEndpoint} {
		b.Run(endpoint, func(b *testing.B) {
			opts := casTestOpts(b)
			sharedURL, legacy := startCASTestServers(b, opts)
			addr := legacy
			if endpoint == casSharedEndpoint {
				addr = sharedURL[len("http://"):]
			}
			client := bytestream.NewByteStreamClient(casTestConn(b, addr, false))
			storage, err := opts.Providers["test"].FromCredentials(context.Background(), "test-secret")
			require.NoError(b, err)
			path := filepath.Join(storage.(*diskBackend).dir, digest)
			up := casTestToken(b, casJWT.Uploader, 0, time.Hour)
			down := casTestToken(b, casJWT.Downloader, 0, time.Hour)
			b.Run("upload", func(b *testing.B) {
				b.SetBytes(size)
				b.ReportAllocs()
				stopSample := sampleCASHeap()
				defer stopSample()
				b.ResetTimer()
				for range b.N {
					b.StopTimer()
					err := os.Remove(path)
					require.True(b, err == nil || os.IsNotExist(err), "%v", err)
					b.StartTimer()
					require.NoError(b, casTestUpload(b, client, up, digest, io.LimitReader(zeroReader{}, size), size))
				}
				b.StopTimer()
				b.ReportMetric(float64(stopSample())/(1<<20), "peak-heap-MiB")
			})
			// Seed downloads independently of the upload benchmark. A sparse
			// file represents the same zero bytes without a heap-sized buffer.
			f, err := os.Create(path)
			require.NoError(b, err)
			require.NoError(b, f.Truncate(size))
			require.NoError(b, f.Close())
			b.Run("download", func(b *testing.B) {
				b.SetBytes(size)
				b.ReportAllocs()
				stopSample := sampleCASHeap()
				defer stopSample()
				b.ResetTimer()
				for range b.N {
					ctx, cancel := casTestContext(down)
					stream, err := client.Read(ctx, &bytestream.ReadRequest{ResourceName: digest})
					require.NoError(b, err)
					hash := sha256.New()
					var received int64
					for {
						msg, err := stream.Recv()
						if errors.Is(err, io.EOF) {
							break
						}
						require.NoError(b, err)
						n, err := hash.Write(msg.GetData())
						require.NoError(b, err)
						received += int64(n)
					}
					cancel()
					require.Equal(b, size, received)
					require.Equal(b, digest, hex.EncodeToString(hash.Sum(nil)))
				}
				b.StopTimer()
				b.ReportMetric(float64(stopSample())/(1<<20), "peak-heap-MiB")
			})
		})
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

// sampleCASHeap reports sampled live heap, not total allocated bytes. Both the
// client and server run here; this is not a server-only RSS or bounded-memory proof.
func sampleCASHeap() func() uint64 {
	runtime.GC()
	done, result := make(chan struct{}), make(chan uint64, 1)
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		var peak uint64
		for {
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			if stats.HeapAlloc > peak {
				peak = stats.HeapAlloc
			}
			select {
			case <-done:
				result <- peak
				return
			case <-ticker.C:
			}
		}
	}()
	return sync.OnceValue(func() uint64 { close(done); return <-result })
}
