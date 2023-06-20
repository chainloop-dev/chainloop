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

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"regexp"

	extensionsSDK "github.com/chainloop-dev/chainloop/app/controlplane/extensions"
	"github.com/go-kratos/kratos/v2/log"
)

const registrationInputHeader = "## Registration Input Schema"
const AttachmentInputHeader = "## Attachment Input Schema"

var registrationInputRe = regexp.MustCompile(registrationInputHeader)
var attachmentInputRe = regexp.MustCompile(AttachmentInputHeader)

var extensionsDir string

func main() {
	l := log.NewStdLogger(os.Stdout)

	extensions, err := extensionsSDK.Load(l)
	if err != nil {
		panic(err)
	}

	for _, e := range extensions {
		// Find README file
		file, err := os.OpenFile(filepath.Join(extensionsDir, e.Describe().ID, "v1", "README.md"), os.O_RDWR, 0644)
		if err != nil {
			_ = l.Log(log.LevelWarn, "msg", "failed to open README.md file", "err", err)
			continue
		}

		fileContent, err := io.ReadAll(file)
		if err != nil {
			_ = l.Log(log.LevelWarn, "msg", "failed to read README.md file", "err", err)
			continue
		}

		// Replace registration input schema
		var prettyRegistrationJSON bytes.Buffer
		err = json.Indent(&prettyRegistrationJSON, e.Describe().RegistrationJSONSchema, "", "  ")
		if err != nil {
			_ = l.Log(log.LevelWarn, "msg", "failed to indent JSON", "err", err)
		}

		fileContent = registrationInputRe.ReplaceAllLiteral(fileContent, []byte(registrationInputHeader+"\n\n```json\n"+prettyRegistrationJSON.String()+"\n```"))
		// Replace attachment schema

		var prettyAttachmentJSON bytes.Buffer
		err = json.Indent(&prettyAttachmentJSON, e.Describe().AttachmentJSONSchema, "", "  ")
		if err != nil {
			panic(err)
		}

		fileContent = attachmentInputRe.ReplaceAllLiteral(fileContent, []byte(AttachmentInputHeader+"\n\n```json\n"+prettyRegistrationJSON.String()+"\n```"))
		// Write the new content in the file
		_, err = file.Seek(0, 0)
		if err != nil {
			_ = l.Log(log.LevelWarn, "msg", "failed to seek README.md file", "err", err)
			continue
		}

		_, err = file.Write(fileContent)
		if err != nil {
			_ = l.Log(log.LevelWarn, "msg", "failed to write README.md file", "err", err)
			continue
		}
	}
}

func init() {
	flag.StringVar(&extensionsDir, "dir", "", "base directory for extensions i.e ./core")
	flag.Parse()
}
