//
// Copyright 2024 The Chainloop Authors.
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

package service

import (
	"fmt"
	"net/http"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/apitoken"

	jwtmiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/gorilla/mux"
	"github.com/prometheus/common/expfmt"
)

const (
	// PrometheusMetricsPath is the path for the Prometheus metrics
	PrometheusMetricsPath = "/prom/{org_name}/metrics"
)

// PrometheusService is the prometheus service
type PrometheusService struct {
	*service
	// Use Cases
	prometheusUseCase   *biz.PrometheusUseCase
	organizationUseCase *biz.OrganizationUseCase
}

// NewPrometheusService creates a new prometheus service
func NewPrometheusService(orgUseCase *biz.OrganizationUseCase, prometheusUseCase *biz.PrometheusUseCase, opts ...NewOpt) *PrometheusService {
	return &PrometheusService{
		organizationUseCase: orgUseCase,
		prometheusUseCase:   prometheusUseCase,
		service:             newService(opts...),
	}
}

// MetricsRequestHandler is the handler for the metrics request. It fetches the Prometheus registry
// and if found, retrieves all metrics in Prometheus format.
func (p *PrometheusService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract org_name from the URL path
	orgName, ok := mux.Vars(r)["org_name"]
	if !ok {
		http.Error(w, "Error extracting organization name from URL path", http.StatusBadRequest)
		return
	}

	// Extracts the organization name from the request
	rawClaims, ok := jwtmiddleware.FromContext(r.Context())
	if !ok {
		http.Error(w, "Error extracting claims from context", http.StatusInternalServerError)
		return
	}

	apiTokenClaims, ok := rawClaims.(*apitoken.CustomClaims)
	if !ok {
		http.Error(w, "Error extracting API Token claims", http.StatusInternalServerError)
		return
	}

	// Check if the organization in the API Token matches the one in the request
	if apiTokenClaims.OrgName != orgName {
		http.Error(w, fmt.Sprintf("Organization [%v] on API Token does not match the organization in the request", apiTokenClaims.OrgName), http.StatusBadRequest)
		return
	}

	// Checks if the organization has a Prometheus integration activated
	if !p.prometheusUseCase.OrganizationHasRegistry(orgName) {
		http.Error(w, "Organization does not have a Prometheus integration activated", http.StatusNotFound)
		return
	}

	// Fetches the Prometheus registry for the organization
	reg := p.prometheusUseCase.GetRegistryByOrganizationName(orgName)
	if reg == nil {
		http.Error(w, "Error fetching Prometheus registry", http.StatusInternalServerError)
		return
	}

	// Gathers the metrics
	gather, err := reg.Gather()
	if err != nil {
		http.Error(w, "Error gathering metrics", http.StatusInternalServerError)
		return
	}

	// Encode and write the metrics to the response
	contentType := expfmt.Negotiate(r.Header)
	w.Header().Set("Content-Type", string(contentType))

	enc := expfmt.NewEncoder(w, contentType)
	for _, mf := range gather {
		if err := enc.Encode(mf); err != nil {
			http.Error(w, "Error encoding metrics", http.StatusInternalServerError)
			return
		}
	}
}
