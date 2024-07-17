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

package middlewareshttp

import (
	nhttp "net/http"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/golang-jwt/jwt/v4"
)

const (
	// bearerWord the bearer key word for authorization
	bearerWord string = "Bearer"
	// authorizationKey holds the key used to store the JWT Token in the request tokenHeader.
	authorizationKey string = "Authorization"
)

// ClaimsFunc is a function that returns a jwt.Claims with the custom claims and correct type
type ClaimsFunc func() jwt.Claims

// AuthFromQueryParam is a middleware that extracts the token from the query parameter and verifies it
func AuthFromQueryParam(keyFunc jwt.Keyfunc, claimsFunc ClaimsFunc, signingMethod jwt.SigningMethod, next nhttp.Handler) nhttp.Handler {
	return nhttp.HandlerFunc(func(w http.ResponseWriter, r *nhttp.Request) {
		token := r.URL.Query().Get("t")
		if token == "" {
			nhttp.Error(w, "missing token", nhttp.StatusUnauthorized)
			return
		}

		claims, err := verifyAndMarshalJWT(token, keyFunc, claimsFunc, signingMethod)
		if err != nil {
			// return unauthorized
			nhttp.Error(w, "invalid token", nhttp.StatusUnauthorized)
			return
		}

		// Attach the claims to the context
		ctx := jwtMiddleware.NewContext(r.Context(), *claims)
		r = r.WithContext(ctx)

		// Run the next handler
		next.ServeHTTP(w, r)
	})
}

// AuthFromAuthorizationHeader is a middleware that extracts the token from the authorization header and verifies it
func AuthFromAuthorizationHeader(keyFunc jwt.Keyfunc, claimsFunc ClaimsFunc, signingMethod jwt.SigningMethod, next nhttp.Handler) nhttp.Handler {
	return nhttp.HandlerFunc(func(w http.ResponseWriter, r *nhttp.Request) {
		auths := strings.SplitN(r.Header.Get(authorizationKey), " ", 2)
		if len(auths) != 2 || !strings.EqualFold(auths[0], bearerWord) {
			nhttp.Error(w, "JWT token is missing", nhttp.StatusUnauthorized)
			return
		}

		jwtToken := auths[1]

		claims, err := verifyAndMarshalJWT(jwtToken, keyFunc, claimsFunc, signingMethod)
		if err != nil {
			// return unauthorized
			nhttp.Error(w, "invalid token", nhttp.StatusUnauthorized)
			return
		}

		// Attach the claims to the context
		ctx := jwtMiddleware.NewContext(r.Context(), *claims)
		r = r.WithContext(ctx)

		// Run the next handler
		next.ServeHTTP(w, r)
	})
}

// verifyAndMarshalJWT verifies the token and returns the map claims
func verifyAndMarshalJWT(token string, keyFunc jwt.Keyfunc, claimsFunc ClaimsFunc, signingMethod jwt.SigningMethod) (*jwt.Claims, error) {
	var tokenInfo *jwt.Token

	tokenInfo, err := jwt.ParseWithClaims(token, claimsFunc(), keyFunc)
	if err != nil {
		var ve *jwt.ValidationError
		if !errors.As(err, &ve) {
			return nil, errors.Unauthorized("UNAUTHORIZED", err.Error())
		}

		if ve.Errors&jwt.ValidationErrorMalformed != 0 {
			return nil, jwtMiddleware.ErrTokenInvalid
		}

		if ve.Errors&(jwt.ValidationErrorExpired) != 0 {
			return nil, jwtMiddleware.ErrTokenExpired
		}

		if ve.Errors&(jwt.ValidationErrorNotValidYet) != 0 {
			return nil, jwtMiddleware.ErrTokenExpired
		}

		return nil, err
	}

	if !tokenInfo.Valid {
		return nil, jwtMiddleware.ErrTokenInvalid
	}

	if tokenInfo.Method != signingMethod {
		return nil, jwtMiddleware.ErrUnSupportSigningMethod
	}

	return &tokenInfo.Claims, nil
}
