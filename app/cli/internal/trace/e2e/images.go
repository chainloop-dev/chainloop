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

package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
)

// mediaSecret is a fake AWS access key that the secret scanner detects. It is
// not a real key. The media image holds it in its base64 form, so a redaction
// that scans media would rewrite the image. It is built from two parts, so
// that the source code holds no key that a scanner of the repository reports.
const mediaSecret = "AKIA" + "QX7RZ2M4KJ6WPL5N"

// pngSignature and the IHDR chunk take the first 33 bytes of a PNG.
const pngHeaderLen = 8 + 25

// solidPNG returns a small PNG of one color.
func solidPNG(c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := range 8 {
		for y := range 8 {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img) // writing to a buffer does not fail
	return buf.Bytes()
}

// pastedPNG is the image that the user pastes in the pasted image scenario.
func pastedPNG() []byte {
	return solidPNG(color.RGBA{R: 200, G: 30, B: 30, A: 255})
}

// mediaPNG returns a valid PNG whose base64 form holds mediaSecret between
// two '/' characters, so that the secret scanner sees a whole word. A tEXt
// chunk after the IHDR chunk carries the bytes. Its keyword has 3 characters,
// so that the chunk data after the keyword starts on a 3-byte boundary of
// the file, where a base64 group starts.
func mediaPNG() []byte {
	secret, _ := base64.StdEncoding.DecodeString(mediaSecret) // a constant of 20 base64 characters
	// 0xFF bytes encode to "////" around the secret.
	text := append(append([]byte("e2e\x00\xff\xff\xff"), secret...), 0xff, 0xff, 0xff)

	var chunk bytes.Buffer
	_ = binary.Write(&chunk, binary.BigEndian, uint32(len(text)))
	chunk.WriteString("tEXt")
	chunk.Write(text)
	_ = binary.Write(&chunk, binary.BigEndian, crc32.ChecksumIEEE(append([]byte("tEXt"), text...)))

	base := solidPNG(color.RGBA{R: 30, G: 30, B: 200, A: 255})
	out := make([]byte, 0, len(base)+chunk.Len())
	out = append(out, base[:pngHeaderLen]...)
	out = append(out, chunk.Bytes()...)
	return append(out, base[pngHeaderLen:]...)
}
