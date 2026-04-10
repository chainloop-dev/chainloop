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

package usercontext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	errorsAPI "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/chainloop-dev/chainloop/pkg/cache"
)

// Operations that require authorization checks
var operationAuthTargets = map[string]bool{
	"/controlplane.v1.OrganizationService/Create": true,
}

type operationAuthRequest struct {
	Operation      string `json:"operation"`
	UserID         string `json:"user_id"`
	OrganizationID string `json:"organization_id,omitempty"`
}

type operationAuthResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

// WithOperationAuthorizationMiddleware forwards specific operations to an external authorization.
// If the external call fails, the request is denied.
func WithOperationAuthorizationMiddleware(conf *conf.OperationAuthorizationProvider, logger *log.Helper) middleware.Middleware {
	if conf == nil || !conf.GetEnabled() || conf.GetUrl() == "" {
		return func(handler middleware.Handler) middleware.Handler {
			return handler
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	url := conf.GetUrl()

	// LRU cache with 30s TTL keyed by "user_id:operation"
	authCache, err := cache.New[*operationAuthResponse](
		cache.WithTTL(30*time.Second),
		cache.WithDescription("Operation authorization cache"),
	)
	if err != nil {
		logger.Warnw("msg", "failed to create operation auth cache, proceeding without cache", "error", err)
		authCache = nil
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			info, ok := transport.FromServerContext(ctx)
			if !ok {
				return handler(ctx, req)
			}

			operation := info.Operation()
			if !operationAuthTargets[operation] {
				return handler(ctx, req)
			}

			user := entities.CurrentUser(ctx)
			if user == nil {
				return handler(ctx, req)
			}

			var orgID string
			if org := entities.CurrentOrg(ctx); org != nil {
				orgID = org.ID
			}

			cacheKey := fmt.Sprintf("%s:%s", user.ID, operation)

			// Check cache
			if authCache != nil {
				if cached, found, _ := authCache.Get(ctx, cacheKey); found {
					if !cached.Allowed {
						return nil, errorsAPI.Forbidden("operation_denied", cached.Reason)
					}
					return handler(ctx, req)
				}
			}

			result, err := callAuthorizationEndpoint(ctx, client, url, &operationAuthRequest{
				Operation:      operation,
				UserID:         user.ID,
				OrganizationID: orgID,
			})
			if err != nil {
				logger.Errorw("msg", "operation authorization call failed, denying request (fail-closed)", "error", err, "operation", operation)
				return nil, errorsAPI.Forbidden("operation_denied", "unable to verify operation authorization")
			}

			// Cache the result
			if authCache != nil {
				_ = authCache.Set(ctx, cacheKey, result)
			}

			if !result.Allowed {
				return nil, errorsAPI.Forbidden("operation_denied", result.Reason)
			}

			return handler(ctx, req)
		}
	}
}

func callAuthorizationEndpoint(ctx context.Context, client *http.Client, url string, authReq *operationAuthRequest) (*operationAuthResponse, error) {
	body, err := json.Marshal(authReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling authorization endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authorization endpoint returned status %d", resp.StatusCode)
	}

	var authResp operationAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &authResp, nil
}
