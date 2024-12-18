package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRegistrationInput(t *testing.T) {
	testCases := []struct {
		name   string
		input  map[string]interface{}
		errMsg string
	}{
		{
			name:   "not ok, missing required property",
			input:  map[string]interface{}{},
			errMsg: "missing properties: 'webhook'",
		},
		{
			name:   "not ok, random properties",
			input:  map[string]interface{}{"foo": "bar"},
			errMsg: "additionalProperties 'foo' not allowed",
		},
		{
			name:  "ok, all properties",
			input: map[string]interface{}{"webhook": "http://repo.io", "username": "u"},
		},
		{
			name:  "ok, only required properties",
			input: map[string]interface{}{"webhook": "http://repo.io"},
		},
		{
			name:   "not ok, empty username",
			input:  map[string]interface{}{"webhook": "http://repo.io", "username": ""},
			errMsg: "length must be >= 1, but got 0",
		},
		{
			name:  "ok, webhook with path",
			input: map[string]interface{}{"webhook": "http://repo/foo/bar"},
		},
		{
			name:   "not ok, invalid webhook, missing protocol",
			input:  map[string]interface{}{"webhook": "repo.io"},
			errMsg: "is not valid 'uri'",
		},
		{
			name:   "not ok, empty webhook",
			input:  map[string]interface{}{"webhook": ""},
			errMsg: "is not valid 'uri'",
		},
	}

	integration, err := New(nil)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.input)
			require.NoError(t, err)

			err = integration.ValidateRegistrationRequest(payload)
			if tc.errMsg != "" {
				assert.ErrorContains(t, err, tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	testCases := []struct {
		name       string
		webhookURL string
		username   string
		wantErr    bool
	}{
		{
			name:       "valid registration",
			webhookURL: "http://example.com",
			username:   "user",
			wantErr:    false,
		},
		{
			name:       "invalid registration, invalid webhook URL",
			webhookURL: "http://invalid-url",
			username:   "user",
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			integration, err := New(log.NewStdLogger(&bytes.Buffer{}))
			require.NoError(t, err)

			req := &sdk.RegistrationRequest{
				Payload: sdk.Configuration([]byte(`{"webhook":"` + tc.webhookURL + `", "username":"` + tc.username + `"}`)),
			}

			resp, err := integration.Register(context.Background(), req)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestAttach(t *testing.T) {
	integration, err := New(log.NewStdLogger(&bytes.Buffer{}))
	require.NoError(t, err)

	req := &sdk.AttachmentRequest{}
	resp, err := integration.Attach(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestExecute(t *testing.T) {
	testCases := []struct {
		name       string
		webhookURL string
		username   string
		wantErr    bool
	}{
		{
			name:       "valid execution",
			webhookURL: "http://example.com",
			username:   "user",
			wantErr:    false,
		},
		{
			name:       "invalid execution, invalid webhook URL",
			webhookURL: "http://invalid-url",
			username:   "user",
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			integration, err := New(log.NewStdLogger(&bytes.Buffer{}))
			require.NoError(t, err)

			req := &sdk.ExecutionRequest{
				RegistrationInfo: &sdk.RegistrationResponse{
					Credentials: &sdk.Credentials{Password: tc.webhookURL},
					Configuration: sdk.Configuration([]byte(`{"name":"webhook", "owner":"owner", "username":"` + tc.username + `"}`)),
				},
				Input: &sdk.ExecuteInput{
					Attestation: &sdk.ExecuteAttestation{
						Envelope: &dsse.Envelope{},
					},
				},
			}

			err = integration.Execute(context.Background(), req)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
