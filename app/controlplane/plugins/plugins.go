//
// Copyright 2023 The Chainloop Authors.
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

package plugins

import (
	"fmt"

	dependencytrack "github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/dependency-track/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/discord-webhook/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/guac/v1"
	ociregistry "github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/oci-registry/v1"
	slack "github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/slack-webhook/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/smtp/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
)

// Load the available third party integrations
// In the future this code will iterate over a dynamic directory of plugins
// and try to load them one by one
func Load(l log.Logger) (sdk.AvailablePlugins, error) {
	// Array of integrations that are meant to be loaded
	// Eventually this will be dynamically loaded from a directory
	toEnable := []sdk.FanOutFactory{
		dependencytrack.New,
		smtp.New,
		ociregistry.New,
		discord.New,
		guac.New,
		slack.New,
	}

	return doLoad(toEnable, l)
}

// doLoad loads the integrations making sure that no duplicates are loaded
func doLoad(plugins []sdk.FanOutFactory, l log.Logger) (sdk.AvailablePlugins, error) {
	logger := servicelogger.ScopedHelper(l, "plugins")

	var res sdk.AvailablePlugins
	var registeredIDs = make(map[string]bool)

	for _, f := range plugins {
		d, err := f(l)
		if err != nil {
			return nil, fmt.Errorf("failed to load plugin: %w", err)
		}

		// NOTE: we do not take into account version yet.
		id := d.Describe().ID
		if _, ok := registeredIDs[id]; ok {
			return nil, fmt.Errorf("there is already an plugin loaded with id %q", id)
		}

		registeredIDs[id] = true

		res = append(res, d)
		logger.Infow("msg", "loaded", "plugin", d.String())
	}

	return res, nil
}
