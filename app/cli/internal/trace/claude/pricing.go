package claude

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// modelPricing holds per-million-token rates in USD for a Claude model.
type modelPricing struct {
	// Input is the price per 1M base input tokens.
	Input float64 `json:"input"`
	// Output is the price per 1M output tokens.
	Output float64 `json:"output"`
	// CacheWrite is the price per 1M tokens written to the 5-minute prompt cache tier (1.25x base input).
	CacheWrite float64 `json:"cache_write"`
	// CacheWrite1h is the price per 1M tokens written to the 1-hour prompt cache tier (2x base input).
	CacheWrite1h float64 `json:"cache_write_1h"`
	// CacheRead is the price per 1M tokens read from the prompt cache (0.1x base input).
	CacheRead float64 `json:"cache_read"`
}

// pricingFile is the schema of the embedded pricing.json data file.
type pricingFile struct {
	Version      string                  `json:"version"`
	DefaultModel string                  `json:"default_model"`
	Models       map[string]modelPricing `json:"models"`
}

//go:embed pricing.json
var pricingJSON []byte

var (
	pricing        map[string]modelPricing
	defaultPricing modelPricing
	pricingVersion string
)

func init() {
	var f pricingFile
	if err := json.Unmarshal(pricingJSON, &f); err != nil {
		panic(fmt.Sprintf("claude trace: invalid embedded pricing.json: %v", err))
	}

	if len(f.Models) == 0 {
		panic("claude trace: embedded pricing.json has no models")
	}

	def, ok := f.Models[f.DefaultModel]
	if !ok {
		panic(fmt.Sprintf("claude trace: default_model %q missing from pricing.json models", f.DefaultModel))
	}

	pricing = f.Models
	defaultPricing = def
	pricingVersion = f.Version
}
