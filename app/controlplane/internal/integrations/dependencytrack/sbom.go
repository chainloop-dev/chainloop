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

package dependencytrack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"encoding/json"

	integration "github.com/chainloop-dev/bedrock/app/controlplane/internal/integrations"
)

type base struct {
	host   *url.URL
	apiKey string
}

type Integration struct {
	*base
	checkAutoCreate bool
}

type SBOMUploader struct {
	*base
	sbom io.Reader
	// Either use a projectID or create a new one by name
	projectID, projectName string
}

var _ integration.Checker = (*Integration)(nil)
var _ integration.Doer = (*SBOMUploader)(nil)

func newBase(host, apiKey string) (*base, error) {
	if apiKey == "" {
		return nil, errors.New("apiKey required")
	}

	uri, err := url.ParseRequestURI(host)
	if err != nil {
		return nil, err
	}

	return &base{host: uri, apiKey: apiKey}, nil
}

// The integration definition
func NewIntegration(host, apiKey string, checkAutoCreate bool) (*Integration, error) {
	b, err := newBase(host, apiKey)
	if err != nil {
		return nil, err
	}

	return &Integration{base: b, checkAutoCreate: checkAutoCreate}, nil
}

func NewSBOMUploader(host, apiKey string, sbom io.Reader, projectID, projectName string) (*SBOMUploader, error) {
	b, err := newBase(host, apiKey)
	if err != nil {
		return nil, err
	}

	if (projectID == "" && projectName == "") || (projectID != "" && projectName != "") {
		return nil, errors.New("either existing project ID or new name is required")
	}

	return &SBOMUploader{b, sbom, projectID, projectName}, nil
}

const bomUploadPermission = "BOM_UPLOAD"

// Required to validate that the provided project exists
const viewPortfolioPermission = "VIEW_PORTFOLIO"
const projectCreationPermission = "PROJECT_CREATION_UPLOAD"

func (d *Integration) Validate(ctx context.Context) error {
	resp, err := teamPermissionsRequest(d.host, d.apiKey)
	if err != nil {
		return err
	}

	teamPermissions := make([]string, 0, len(resp.Permissions))
	for _, p := range resp.Permissions {
		teamPermissions = append(teamPermissions, p.Name)
	}

	return doCheck(teamPermissions, d.checkAutoCreate)
}

// Validate before uploading an sbom
// This method will take into account validations from two different life-cycles
// - Validation used to make sure that the dependency track instance is correctly setup
// - That the provided parameters, i.e project_id is valid, meaning it exists in the instance
func (d *SBOMUploader) Validate(ctx context.Context) error {
	autocreate := d.projectName != "" && d.projectID == ""
	// Check auto-create permissions
	integration, err := NewIntegration(d.host.String(), d.apiKey, autocreate)
	if err != nil {
		return fmt.Errorf("intializing permissions checker: %w", err)
	}

	if err := integration.Validate(ctx); err != nil {
		return fmt.Errorf("validating the permissions: %w", err)
	}

	if d.projectID == "" {
		return nil
	}

	// Check if the project exists
	var projectFound bool
	projects, err := listProjects(d.host, d.apiKey)
	if err != nil {
		return fmt.Errorf("checking that the project exists: %w", err)
	}

	for _, p := range projects {
		if p.ID == d.projectID {
			projectFound = true
			break
		}
	}

	if !projectFound {
		return fmt.Errorf("project with ID %q not found", d.projectID)
	}

	return nil
}

func (d *SBOMUploader) Do(ctx context.Context) error {
	// Now we know that we can upload
	values := map[string]io.Reader{
		"bom": d.sbom,
	}

	autocreate := d.projectName != "" && d.projectID == ""
	if autocreate {
		values["autoCreate"] = strings.NewReader("true")
		values["projectName"] = strings.NewReader(d.projectName)
	} else {
		values["project"] = strings.NewReader(d.projectID)
	}

	_, err := uploadSBOMRequest(d.host, d.apiKey, values)
	return err
}

func doCheck(teamPermissions []string, autoCreate bool) error {
	// Required set of permissions to find in the response
	found := map[string]bool{
		bomUploadPermission:     false,
		viewPortfolioPermission: false,
	}

	if autoCreate {
		found[projectCreationPermission] = false
	}

	for _, p := range teamPermissions {
		if _, ok := found[p]; ok {
			found[p] = true
		}
	}

	for name, found := range found {
		if !found {
			return fmt.Errorf("%w: permission: %s", ErrValidation, name)
		}
	}

	return nil
}

type teamPermissionsResponse struct {
	UUID        string
	Name        string
	Permissions []struct {
		Name, Description string
	}
}

func teamPermissionsRequest(host *url.URL, apiKey string) (*teamPermissionsResponse, error) {
	apiEndpoint := host.JoinPath("/api/v1/team/self")

	req, err := http.NewRequest(http.MethodGet, apiEndpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", apiKey)
	// Submit the request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Check the response
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
		return nil, err
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	resp := &teamPermissionsResponse{}
	if err := json.Unmarshal(resBody, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

var ErrValidation = errors.New("validation error")

type uploadSBOMResponse struct {
	Token string
}

func uploadSBOMRequest(host *url.URL, apiKey string, values map[string]io.Reader) (*uploadSBOMResponse, error) {
	// Prepare the form-data
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for key, r := range values {
		var fw io.Writer
		var err error
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}

		if fw, err = w.CreateFormField(key); err != nil {
			return nil, err
		}

		if _, err := io.Copy(fw, r); err != nil {
			return nil, err
		}
	}

	w.Close()

	// Prepare request
	apiEndpoint := host.JoinPath("/api/v1/bom")
	req, err := http.NewRequest(http.MethodPost, apiEndpoint.String(), &b)
	if err != nil {
		return nil, err
	}

	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Api-Key", apiKey)

	// Submit the request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Check the response
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
		return nil, err
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	resp := &uploadSBOMResponse{}
	if err := json.Unmarshal(resBody, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

type listProjectsResponseItem struct {
	ID   string `json:"uuid"`
	Name string
}

type listProjectsResponse []listProjectsResponseItem

// We are listing projects instead of accessing a specific one to enable
// son in the future listing and selection in the UI
func listProjects(host *url.URL, apiKey string) (listProjectsResponse, error) {
	apiEndpoint := host.JoinPath("/api/v1/project")

	req, err := http.NewRequest(http.MethodGet, apiEndpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", apiKey)
	// Submit the request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Check the response
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
		return nil, err
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	resp := make([]listProjectsResponseItem, 0)
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	return resp, nil
}
