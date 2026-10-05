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

//go:build windows

package cmd

import "syscall"

// detachedProcess is CREATE_NEW_PROCESS_GROUP's companion flag, DETACHED_PROCESS, which
// the syscall package does not define. It is spelled out here rather than pulled from
// golang.org/x/sys/windows so that module stays an indirect dependency instead of becoming
// a direct one for the sake of a single constant.
// https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
const detachedProcess = 0x00000008

// detachSysProcAttr gives the telemetry child its own process group and no console, so a
// Ctrl-C in the parent's console and the console's teardown do not reach it. Windows is
// not a release target for the CLI, so this is best effort: it keeps `go install` builds
// working and behaving sensibly rather than being tuned for it.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess,
	}
}
