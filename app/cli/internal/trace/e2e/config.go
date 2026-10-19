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
	_ "embed"
	"errors"
	"fmt"
	"time"

	"sigs.k8s.io/yaml"
)

//go:embed config.yaml
var defaultConfig []byte

// HarnessConfig pins a harness column and lists its models.
type HarnessConfig struct {
	// Version is the pinned harness version (D-013).
	Version string `json:"version"`
	// Models are the models that the suite runs the harness with (D-009).
	Models []string `json:"models"`
}

// Repeat sets how many times a cell runs and how many runs must pass.
type Repeat struct {
	Runs      int `json:"runs"`
	Threshold int `json:"threshold"`
}

func (r Repeat) validate() error {
	if r.Runs < 1 || r.Threshold < 1 || r.Threshold > r.Runs {
		return fmt.Errorf("runs %d and threshold %d: want 1 <= threshold <= runs", r.Runs, r.Threshold)
	}
	return nil
}

// Config is the configuration of the suite.
type Config struct {
	// Harnesses are keyed by the column name in the feature table.
	Harnesses map[string]HarnessConfig `json:"harnesses"`
	// ModelDriven is the default repeat of a model-driven feature (R-006).
	ModelDriven Repeat `json:"modelDriven"`
	// Features override the repeat of a feature, keyed by feature name.
	Features        map[string]Repeat `json:"features"`
	ScenarioTimeout Duration          `json:"scenarioTimeout"`
	Parallel        int               `json:"parallel"`
}

// Duration is a time.Duration written as a Go duration string in YAML.
type Duration time.Duration

// UnmarshalJSON reads a duration string such as "5m".
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

// LoadConfig reads the suite configuration. A nil data reads the embedded
// config.yaml.
func LoadConfig(data []byte) (*Config, error) {
	if data == nil {
		data = defaultConfig
	}

	var cfg Config
	if err := yaml.UnmarshalStrict(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode suite config: %w", err)
	}

	if len(cfg.Harnesses) == 0 {
		return nil, errors.New("suite config: no harness")
	}
	for name, h := range cfg.Harnesses {
		if h.Version == "" || len(h.Models) == 0 {
			return nil, fmt.Errorf("suite config: harness %q needs a version and at least one model", name)
		}
	}
	if err := cfg.ModelDriven.validate(); err != nil {
		return nil, fmt.Errorf("suite config: modelDriven: %w", err)
	}
	for name, r := range cfg.Features {
		if err := r.validate(); err != nil {
			return nil, fmt.Errorf("suite config: feature %q: %w", name, err)
		}
	}
	if cfg.ScenarioTimeout <= 0 {
		return nil, errors.New("suite config: scenarioTimeout must be positive")
	}
	if cfg.Parallel < 1 {
		cfg.Parallel = 1
	}

	return &cfg, nil
}

// repeatFor returns how many times a feature runs and how many runs must pass.
func (c *Config) repeatFor(feature string, modelDriven bool) Repeat {
	if r, ok := c.Features[feature]; ok {
		return r
	}
	if modelDriven {
		return c.ModelDriven
	}
	return Repeat{Runs: 1, Threshold: 1}
}
