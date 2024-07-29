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

package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	jwtmiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

// Mock Keyfunc
func mockKeyFunc(_ *jwt.Token) (interface{}, error) {
	return []byte("secret"), nil
}

// Mock signing method
var mockSigningMethod = jwt.SigningMethodHS256

// Helper function to generate a valid token with custom claims
func generateValidToken() string {
	token := jwt.NewWithClaims(mockSigningMethod, jwt.MapClaims{"foo": "bar"})
	tokenString, _ := token.SignedString([]byte("secret"))
	return tokenString
}

func genericClaimsFunc() ClaimsFunc {
	return func() jwt.Claims {
		return &jwt.MapClaims{}
	}
}

func TestAuthFromQueryParam(t *testing.T) {
	validToken := generateValidToken()

	tests := []struct {
		name       string
		token      string
		wantStatus int
		wantClaims map[string]interface{}
	}{
		{"Valid Token", validToken, http.StatusOK, map[string]interface{}{"foo": "bar"}},
		{"Missing Token", "", http.StatusUnauthorized, nil},
		{"Invalid Token", "invalidtoken", http.StatusUnauthorized, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/?t="+tt.token, nil)
			rr := httptest.NewRecorder()
			handler := AuthFromQueryParam(mockKeyFunc, genericClaimsFunc(), mockSigningMethod, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				claims, ok := jwtmiddleware.FromContext(r.Context())
				mapClaims := claims.(*jwt.MapClaims)
				if tt.wantClaims != nil {
					assert.True(t, ok, "claims not found in context")
					for key, value := range tt.wantClaims {
						claimsVal, exists := (*mapClaims)[key]
						assert.True(t, exists, "claims missing key: %v", key)
						assert.Equal(t, value, claimsVal, "claims value mismatch for key: %v", key)
					}
				}
				w.WriteHeader(http.StatusOK)
			}))
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code, "handler returned wrong status code")
		})
	}
}

func TestAuthFromAuthorizationHeader(t *testing.T) {
	validToken := generateValidToken()

	tests := []struct {
		name       string
		header     string
		wantStatus int
		wantClaims map[string]interface{}
	}{
		{"Valid Token", "Bearer " + validToken, http.StatusOK, map[string]interface{}{"foo": "bar"}},
		{"Missing Header", "", http.StatusUnauthorized, nil},
		{"Invalid Header Format", "Bearer", http.StatusUnauthorized, nil},
		{"Invalid Token", "Bearer invalidtoken", http.StatusUnauthorized, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set(authorizationKey, tt.header)
			}
			rr := httptest.NewRecorder()
			handler := AuthFromAuthorizationHeader(mockKeyFunc, genericClaimsFunc(), mockSigningMethod, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				claims, ok := jwtmiddleware.FromContext(r.Context())
				mapClaims := claims.(*jwt.MapClaims)
				if tt.wantClaims != nil {
					assert.True(t, ok, "claims not found in context")
					for key, value := range tt.wantClaims {
						claimsVal, exists := (*mapClaims)[key]
						assert.True(t, exists, "claims missing key: %v", key)
						assert.Equal(t, value, claimsVal, "claims value mismatch for key: %v", key)
					}
				}
				w.WriteHeader(http.StatusOK)
			}))
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code, "handler returned wrong status code")
		})
	}
}

// CustomClaimsA represents the first custom JWT claims type
type CustomClaimsA struct {
	jwt.RegisteredClaims
	Foo string `json:"foo"`
}

// CustomClaimsB represents the second custom JWT claims type
type CustomClaimsB struct {
	jwt.RegisteredClaims
	Bar int `json:"bar"`
}

// Helper function to generate a valid token with CustomClaimsA
func generateValidTokenA() string {
	claims := CustomClaimsA{
		Foo: "bar",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "test",
		},
	}
	token := jwt.NewWithClaims(mockSigningMethod, claims)
	tokenString, _ := token.SignedString([]byte("secret"))
	return tokenString
}

// Helper function to generate a valid token with CustomClaimsB
func generateValidTokenB() string {
	claims := CustomClaimsB{
		Bar: 42,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "test",
		},
	}
	token := jwt.NewWithClaims(mockSigningMethod, claims)
	tokenString, _ := token.SignedString([]byte("secret"))
	return tokenString
}

func TestAuthWithCustomClaimsConversions(t *testing.T) {
	validTokenA := generateValidTokenA()
	validTokenB := generateValidTokenB()

	tests := []struct {
		name       string
		header     string
		first      bool
		second     bool
		claimsFunc ClaimsFunc
		wantStatus int
		wantClaims interface{}
	}{
		{"Valid Token A", "Bearer " + validTokenA, true, false, func() jwt.Claims { return &CustomClaimsA{} }, http.StatusOK, &CustomClaimsA{Foo: "bar"}},
		{"Valid Token B", "Bearer " + validTokenB, false, true, func() jwt.Claims { return &CustomClaimsB{} }, http.StatusOK, &CustomClaimsB{Bar: 42}},
		{"Missing Header", "", false, false, genericClaimsFunc(), http.StatusUnauthorized, nil},
		{"Invalid Header Format", "Bearer", false, false, genericClaimsFunc(), http.StatusUnauthorized, nil},
		{"Invalid Token", "Bearer invalidtoken", false, false, genericClaimsFunc(), http.StatusUnauthorized, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set(authorizationKey, tt.header)
			}
			rr := httptest.NewRecorder()
			handler := AuthFromAuthorizationHeader(mockKeyFunc, tt.claimsFunc, mockSigningMethod, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				claims, _ := jwtmiddleware.FromContext(r.Context())
				if tt.wantClaims != nil {
					assert.NotNil(t, claims, "claims not found in context")
					if tt.first {
						actual, ok := claims.(*CustomClaimsA)
						assert.True(t, ok, "claims type mismatch: expected CustomClaimsA")
						assert.Equal(t, tt.wantClaims.(*CustomClaimsA).Foo, actual.Foo, "claims value mismatch for key: Foo")
					} else if tt.second {
						actual, ok := claims.(*CustomClaimsB)
						assert.True(t, ok, "claims type mismatch: expected CustomClaimsB")
						assert.Equal(t, tt.wantClaims.(*CustomClaimsB).Bar, actual.Bar, "claims value mismatch for key: Bar")
					}

					switch expected := tt.wantClaims.(type) {
					case *CustomClaimsA:
						actual, ok := claims.(*CustomClaimsA)
						assert.True(t, ok, "claims type mismatch: expected CustomClaimsA")
						assert.Equal(t, expected.Foo, actual.Foo, "claims value mismatch for key: Foo")
					case *CustomClaimsB:
						actual, ok := claims.(*CustomClaimsB)
						assert.True(t, ok, "claims type mismatch: expected CustomClaimsB")
						assert.Equal(t, expected.Bar, actual.Bar, "claims value mismatch for key: Bar")
					}
				}
				w.WriteHeader(http.StatusOK)
			}))
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code, "handler returned wrong status code")
		})
	}
}
