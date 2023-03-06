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

package cmd

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"os"

	"github.com/spf13/cobra"
)

const devVersion = "dev"

var Version = devVersion

type info struct {
	Version string
	Digest  string
}

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Command line version",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("%s version %s\n", appName, Version)
		},
	}
}

func executableInfo() (*info, error) {
	h, err := executableDigest()
	if err != nil {
		return nil, err
	}

	return &info{
		Version: Version,
		Digest:  fmt.Sprintf("sha256:%x", h.Sum(nil)),
	}, nil
}

func executableDigest() (hash.Hash, error) {
	ex, err := os.Executable()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(ex)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}

	return h, nil
}
