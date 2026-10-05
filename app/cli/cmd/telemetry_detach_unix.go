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

//go:build !windows

package cmd

import "syscall"

// detachSysProcAttr puts the telemetry child in a new session, which detaches it from the
// controlling terminal and takes it out of the parent's process group. That is what keeps
// it alive for the second or so it needs after the parent exits: a SIGHUP on terminal
// close, the shell's job control, and a CI runner killing the step's process group all
// address the group the child has just left.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
