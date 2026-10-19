//
// Copyright 2023-2026 The Chainloop Authors.
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
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/internal/oauth"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

const (
	getDownloadURLOperation = "/controlplane.v1.CASRedirectService/DownloadRedirect"

	// bearerFormat authorization token format
	bearerFormat string = "Bearer %s"

	// authorizationKey holds the key used to store the JWT Token in the request tokenHeader.
	authorizationKey string = "Authorization"

	cookieDownloadVerifierPrefix = "downloadVerifier_"
	queryParamDownloadFlow       = "download_flow"
)

type CASRedirectService struct {
	pb.UnimplementedCASRedirectServiceServer
	*service

	casMappingUC    *biz.CASMappingUseCase
	casCredsUseCase *biz.CASCredentialsUseCase
	casServerConf   *conf.Bootstrap_CASServer
	auth            *AuthService
}

func NewCASRedirectService(casmUC *biz.CASMappingUseCase, casCredsUC *biz.CASCredentialsUseCase, conf *conf.Bootstrap_CASServer, auth *AuthService, opts ...NewOpt) (*CASRedirectService, error) {
	if conf == nil || conf.GetDownloadUrl() == "" {
		return nil, errors.New("CASServer.downloadURL configuration is missing")
	}
	if auth == nil {
		return nil, errors.New("download authentication service is missing")
	}

	return &CASRedirectService{
		service:         newService(opts...),
		casMappingUC:    casmUC,
		casCredsUseCase: casCredsUC,
		casServerConf:   conf,
		auth:            auth,
	}, nil
}

// GetDownloadURL returns a signed URL to download the artifact from the CAS backend
// The URL includes a JWT token that is used to authenticate the request, this token has all the information required to validate the request
// The result would look like "https://cas.chainloop.dev/download/sha256:[DIGEST]?t=tokenJWT
func (s *CASRedirectService) GetDownloadURL(ctx context.Context, req *pb.GetDownloadURLRequest) (*pb.GetDownloadURLResponse, error) {
	currentUser, currentAPIToken, err := requireCurrentUserOrAPIToken(ctx)
	if err != nil {
		return nil, err
	}

	currentOrg, err := requireCurrentOrg(ctx)
	if err != nil {
		return nil, err
	}

	var mapping *biz.CASMapping
	if currentUser != nil {
		mapping, err = s.casMappingUC.FindCASMappingForDownloadByUser(ctx, req.Digest, currentUser.ID)
	} else if currentAPIToken != nil {
		var orgID uuid.UUID
		orgID, err = uuid.Parse(currentOrg.ID)
		if err != nil {
			return nil, handleUseCaseErr(ctx, err, s.log)
		}
		mapping, err = s.casMappingUC.FindCASMappingForDownloadByOrg(ctx, req.Digest, []uuid.UUID{orgID}, s.rbacScopesForOrg(ctx, orgID))
	}

	if err != nil {
		// We don't want to leak the fact that the asset exists but the user does not have permissions
		// that's why we return a generic 404 in unauthorized scenarios too
		if biz.IsNotFound(err) {
			return nil, kerrors.NotFound("not found", "artifact not found")
		} else if biz.IsErrValidation(err) {
			return nil, kerrors.BadRequest("invalid", err.Error())
		}

		return nil, handleUseCaseErr(ctx, err, s.log)
	}

	backend := mapping.CASBackend

	// inline backends don't have a download URL
	if backend.Inline {
		return nil, kerrors.NotFound("not found", "CAS backend is inline")
	}

	// check if the backend is on a valid state, if not return an error
	if backend.ValidationStatus != biz.CASBackendValidationOK {
		return nil, pb.ErrorCasBackendErrorReasonInvalid("CAS Storage is in an invalid state and can't download artifacts, please fix it before attempting it again")
	}

	// Create an URL to download the artifact from the CAS backend
	downloadBase, err := url.Parse(s.casServerConf.GetDownloadUrl())
	if err != nil {
		return nil, handleUseCaseErr(ctx, err, s.log)
	}

	// 1 - append the digest /download/[digest]
	downloadURL := downloadBase.JoinPath(req.Digest)

	// 2- add authentication token to the query params ?t=[token]
	if backend.SecretName != "" {
		ref := &biz.CASCredsOpts{BackendType: string(backend.Provider), SecretPath: backend.SecretName, Role: casJWT.Downloader, MaxBytes: backend.Limits.MaxBytes, OrgID: backend.OrganizationID}
		t, err := s.casCredsUseCase.GenerateTemporaryCredentials(ref)
		if err != nil {
			return nil, handleUseCaseErr(ctx, err, s.log)
		}

		q := downloadURL.Query()
		q.Set("t", t)
		downloadURL.RawQuery = q.Encode()
	}

	return &pb.GetDownloadURLResponse{Result: &pb.GetDownloadURLResponse_Result{Url: downloadURL.String()}}, nil
}

// Custom HTTP handler that handles redirect to the CAS download URL
// This handler does
// 1 - Force the user to do an oauth login dance
// 2 - calls GetDownloadURL to generate a proper URL
// 3 - redirects to the generated URL
// NOTE: This code is similar to the one that gets autogenerated if we happened to use proto.http but we can't use it because we need custom redirection
func (s *CASRedirectService) HTTPDownload(ctx khttp.Context) error {
	if redirected, err := s.authenticateUser(ctx.Response(), ctx.Request()); err != nil {
		return err
	} else if redirected {
		return nil
	}

	// Arguments marshalling
	var in pb.GetDownloadURLRequest
	if err := ctx.BindVars(&in); err != nil {
		return err
	}

	// Run the downloadURLCode within the middleware chain
	// Set identifier so it can be skipped, filtered, selected during the middleware chain
	khttp.SetOperation(ctx, getDownloadURLOperation)
	h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
		return s.GetDownloadURL(ctx, req.(*pb.GetDownloadURLRequest))
	})

	out, err := h(ctx, &in)
	if err != nil {
		return err
	}

	urlResponse := out.(*pb.GetDownloadURLResponse)
	if urlResponse.GetResult().GetUrl() == "" {
		return kerrors.BadRequest("invalid URL", "the URL returned by the server is empty")
	}

	// if the client is not a browser (i.e curl), we perform a redirect
	acceptHeader := ctx.Request().Header.Get("Accept")
	if !strings.Contains(acceptHeader, "text/http") {
		ctx.Response().Header().Set("Location", urlResponse.Result.Url)
		ctx.Response().WriteHeader(http.StatusFound)
		return nil
	}

	// perform redirect
	// In order ton show a redirect message in the page before the redirection,
	// we need to use a Refresh Header instead of a vanilla redirect (via location change)
	// We want to show a message because in some cases the download will be shown as downloadable and hence
	// the UX for the user is like the browser got stuck
	ctx.Response().Header().Set("Refresh", "1;url="+urlResponse.Result.Url)
	ctx.Response().WriteHeader(http.StatusFound)
	fmt.Fprintln(ctx.Response(), "Your download will begin shortly...")

	return nil
}

// isDownloadCallback recognizes the relative callback used by the browser download route.
func isDownloadCallback(callback string) bool {
	if !isRelativePath(callback) {
		return false
	}
	u, err := url.Parse(callback)
	if err != nil || !strings.HasPrefix(u.Path, "/download/") {
		return false
	}
	digest := strings.TrimPrefix(u.Path, "/download/")
	return digest != "" && !strings.Contains(digest, "/") && digest != "." && digest != ".."
}

// authenticateUser exchanges a browser-bound handoff internally, keeping the user JWT out
// of URLs. Header-authenticated clients continue through the normal middleware chain.
func (s *CASRedirectService) authenticateUser(w http.ResponseWriter, r *http.Request) (bool, error) {
	setTokenLeakHeaders(w)
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return false, kerrors.BadRequest("invalid query", "invalid download query")
	}
	if query.Has(oauth.QueryParamLegacyToken) {
		return false, kerrors.BadRequest("legacy download token", "please open the download link again without the token")
	}
	if r.Header.Get(authorizationKey) != "" {
		return false, nil
	}

	callback := *r.URL
	callback.Scheme, callback.Host, callback.Fragment = "", "", ""
	callback.ForceQuery = false
	query.Del(oauth.QueryParamHandoff)
	callback.RawQuery = query.Encode()
	if !isDownloadCallback(callback.String()) {
		return false, kerrors.BadRequest("invalid callback", "invalid download callback")
	}

	if codes, ok := r.URL.Query()[oauth.QueryParamHandoff]; ok {
		flows := query[queryParamDownloadFlow]
		if len(flows) != 1 {
			return false, errLoginHandoffExchange
		}
		flowID, err := uuid.Parse(flows[0])
		if err != nil || flowID.String() != flows[0] {
			return false, errLoginHandoffExchange
		}
		verifier, err := r.Cookie(cookieDownloadVerifierPrefix + flows[0])
		s.setDownloadVerifierCookie(w, flows[0], "", true)
		if err != nil || len(codes) != 1 || codes[0] == "" {
			return false, errLoginHandoffExchange
		}
		token, err := s.auth.ExchangeHandoff(r.Context(), &pb.AuthServiceExchangeHandoffRequest{
			Handoff: codes[0], CodeVerifier: verifier.Value, Callback: callback.String(),
		})
		if err != nil {
			return false, err
		}
		r.Header.Set(authorizationKey, fmt.Sprintf(bearerFormat, token.Token))
		return false, nil
	}

	if query.Has(queryParamDownloadFlow) {
		return false, errLoginHandoffExchange
	}
	// Bind the verifier cookie and callback to this flow so overlapping downloads
	// neither replace each other's verifier nor clear each other's cookie.
	flowID := uuid.NewString()
	query.Set(queryParamDownloadFlow, flowID)
	callback.RawQuery = query.Encode()
	verifier := oauth2.GenerateVerifier()
	s.setDownloadVerifierCookie(w, flowID, verifier, false)
	loginURL := url.URL{Path: AuthLoginPath}
	loginURL.RawQuery = url.Values{
		oauth.QueryParamCallback:            {callback.String()},
		oauth.QueryParamCodeChallenge:       {oauth2.S256ChallengeFromVerifier(verifier)},
		oauth.QueryParamCodeChallengeMethod: {oauth.CodeChallengeMethodS256},
	}.Encode()
	http.Redirect(w, r, loginURL.String(), http.StatusTemporaryRedirect)
	return true, nil
}

func (s *CASRedirectService) setDownloadVerifierCookie(w http.ResponseWriter, flowID, verifier string, remove bool) {
	// #nosec G124 -- Secure is disabled only in development mode, as for the OAuth state cookies.
	cookie := &http.Cookie{
		Name: cookieDownloadVerifierPrefix + flowID, Value: verifier, Path: "/download/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: 600, Expires: time.Now().Add(10 * time.Minute),
	}
	if s.auth.devMode {
		cookie.Secure = false
	}
	if remove {
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0)
	}
	http.SetCookie(w, cookie)
}
