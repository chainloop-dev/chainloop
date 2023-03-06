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

package service

import (
	"context"

	pb "github.com/chainloop-dev/bedrock/app/controlplane/api/controlplane/v1"
)

type StatusService struct {
	loginURL, version string
	pb.UnimplementedStatusServiceServer
}

func NewStatusService(logingURL, version string) *StatusService {
	return &StatusService{loginURL: logingURL, version: version}
}

func (s *StatusService) Statusz(ctx context.Context, req *pb.StatuszRequest) (*pb.StatuszResponse, error) {
	return &pb.StatuszResponse{}, nil
}

func (s *StatusService) Infoz(ctx context.Context, req *pb.InfozRequest) (*pb.InfozResponse, error) {
	return &pb.InfozResponse{LoginUrl: s.loginURL, Version: s.version}, nil
}
