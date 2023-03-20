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

	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
)

type StatusService struct {
	version string
	pb.UnimplementedStatusServiceServer
}

func NewStatusService(version string) *StatusService {
	return &StatusService{version: version}
}

func (s *StatusService) Statusz(_ context.Context, _ *pb.StatuszRequest) (*pb.StatuszResponse, error) {
	return &pb.StatuszResponse{}, nil
}

func (s *StatusService) Infoz(_ context.Context, _ *pb.InfozRequest) (*pb.InfozResponse, error) {
	return &pb.InfozResponse{Version: s.version}, nil
}
