package claude

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPricingEmbedLoads(t *testing.T) {
	require.NotEmpty(t, pricingVersion, "pricingVersion should be populated from pricing.json")
	require.NotEmpty(t, pricing, "pricing map should be populated from pricing.json")

	assert.NotZero(t, defaultPricing.Input, "defaultPricing should resolve to a non-zero entry")
	assert.NotZero(t, defaultPricing.Output)

	opus, ok := pricing["claude-opus-4-7"]
	require.True(t, ok, "expected claude-opus-4-7 in embedded pricing table")
	assert.Equal(t, 5.00, opus.Input)
	assert.Equal(t, 25.00, opus.Output)
	assert.Equal(t, 6.25, opus.CacheWrite)
	assert.Equal(t, 10.00, opus.CacheWrite1h)
	assert.Equal(t, 0.50, opus.CacheRead)

	opus5, ok := pricing["claude-opus-5"]
	require.True(t, ok, "expected claude-opus-5 in embedded pricing table")
	assert.Equal(t, 5.00, opus5.Input)
	assert.Equal(t, 25.00, opus5.Output)

	sonnet5, ok := pricing["claude-sonnet-5"]
	require.True(t, ok, "expected claude-sonnet-5 in embedded pricing table")
	assert.Equal(t, 2.00, sonnet5.Input)
	assert.Equal(t, 10.00, sonnet5.Output)
	assert.Equal(t, 2.50, sonnet5.CacheWrite)
	assert.Equal(t, 4.00, sonnet5.CacheWrite1h)
	assert.Equal(t, 0.20, sonnet5.CacheRead)
}
