package materials

import (
	"bytes"
	"encoding/json"
	"fmt"

	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

// JsonEnvelopeWithDigest returns the JSON content of the envelope and its digest.
func JsonEnvelopeWithDigest(envelope *dsse.Envelope) ([]byte, cr_v1.Hash, error) {
	jsonContent, err := json.Marshal(envelope)
	if err != nil {
		return nil, cr_v1.Hash{}, fmt.Errorf("marshaling the envelope: %w", err)
	}

	h, _, err := cr_v1.SHA256(bytes.NewBuffer(jsonContent))
	if err != nil {
		return nil, cr_v1.Hash{}, fmt.Errorf("calculating the digest: %w", err)
	}

	return jsonContent, h, nil
}
