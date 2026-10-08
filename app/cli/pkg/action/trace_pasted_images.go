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

package action

import (
	"fmt"
	"net/http"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/pointer"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// imageExtensions gives the file extension of the stored file of a pasted
// image, from its media type.
var imageExtensions = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// pastedImageCaptures returns a capture for each distinct image that the user
// pasted into the session, from the data in the transcript (spec issue-3569).
// The push stores each one as an image spec source, so that the transcript
// can hold a pointer in its place.
//
// An image whose digest is already a source gets no capture: the entry of the
// agent, with its role, title and description, wins.
func pastedImageCaptures(provider trace.Provider, evidence *aicodingsession.Evidence, sources pointer.Sources) []spec.Capture {
	finder, ok := provider.(trace.PastedImageFinder)
	if !ok {
		return nil
	}

	var captures []spec.Capture
	for _, img := range finder.PastedImages(evidence.Data.RawSession) {
		if _, ok := sources[img.Digest]; ok {
			continue
		}

		// The CLI sets no role, no description and no source address: it
		// does not guess them. The title is the label the user saw.
		captures = append(captures, spec.Capture{
			FileName:   fmt.Sprintf("pasted-image-%d%s", img.Number, imageExtension(img)),
			Kind:       aicodingsession.SpecKindImage,
			Meta:       spec.Meta{Title: fmt.Sprintf("Pasted image %d", img.Number)},
			CapturedAt: img.Timestamp,
			Raw:        img.Data,
			Verbatim:   true,
		})
	}

	return captures
}

// imageExtension returns the extension of the stored file of a pasted image,
// from its media type, or from its content when the agent recorded none that
// is known.
func imageExtension(img trace.PastedImage) string {
	if ext, ok := imageExtensions[img.MediaType]; ok {
		return ext
	}

	return imageExtensions[http.DetectContentType(img.Data)]
}
