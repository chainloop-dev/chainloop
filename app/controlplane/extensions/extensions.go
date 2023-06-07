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

package extensions

import (
	"github.com/chainloop-dev/chainloop/app/controlplane/extensions/core/dependencytrack/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/extensions/sdk/v1"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
)

// Load the available third party integrations
// In the future this code will iterate over a dynamic directory of plugins
// and try to load them one by one
func Load(l log.Logger) (sdk.Loaded, error) {
	var res sdk.Loaded

	// Array of integrations that are meant to be loaded
	// Eventually this will be dynamically loaded from a directory
	toEnable := []sdk.FanOutFactory{
		dependencytrack.New,
	}

	// Initialize and load the extensions
	logger := servicelogger.ScopedHelper(l, "extensions")
	for _, f := range toEnable {
		d, err := f(l)
		if err != nil {
			logger.Errorw("failed to load extension", "error", err.Error())
			continue
		}

		logger.Infow("msg", "loaded", "extension", d.String())

		res = append(res, d)
	}

	return res, nil
}
