package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "long path is masked with last 4 visible",
			raw:  "https://example.com/some/long/path/with/many/segments/secret-token",
			want: "https://example.com****oken",
		},
		{
			name: "short path is not masked",
			raw:  "https://example.com/sho",
			want: "https://example.com/sho",
		},
		{
			name: "path at threshold is not masked",
			raw:  "https://example.com/abc",
			want: "https://example.com/abc",
		},
		{
			name: "path above threshold is masked",
			raw:  "https://example.com/abcde",
			want: "https://example.com****bcde",
		},
		{
			name: "empty string returns empty",
			raw:  "",
			want: "",
		},
		{
			name: "missing host returns empty",
			raw:  "/just/a/path",
			want: "",
		},
		{
			name: "webhook URL with port",
			raw:  "https://prod-00.westus.logic.azure.com:443/workflows/1234567890abcdef/triggers/manual/paths/invoke",
			want: "https://prod-00.westus.logic.azure.com:443****voke",
		},
		{
			name: "no path",
			raw:  "https://example.com",
			want: "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskURL(tt.raw)
			assert.Equal(t, tt.want, got)
		})
	}
}
