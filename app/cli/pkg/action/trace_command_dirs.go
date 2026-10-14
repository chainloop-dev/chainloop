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

package action

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	// shellAssignment matches a leading variable assignment ("GIT_TRACE=1 git …").
	shellAssignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

	// gitValueOptions are the git options before the subcommand that take
	// their value as the next word.
	gitValueOptions = []string{"-c", "--git-dir", "--work-tree", "--namespace", "--config-env"}

	// gitCloneValueOptions are the git clone options that take their value as
	// the next word.
	gitCloneValueOptions = []string{
		"-o", "--origin", "-b", "--branch", "-u", "--upload-pack", "--reference",
		"--reference-if-able", "--separate-git-dir", "--depth", "--shallow-since",
		"--shallow-exclude", "-c", "--config", "-j", "--jobs", "--template",
		"--filter", "--server-option", "--bundle-uri", "--ref-format",
	}

	// ghCloneValueOptions are the gh repo clone options that take their value
	// as the next word.
	ghCloneValueOptions = []string{"-u", "--upstream-remote-name"}
)

// commandCheckoutDirs returns the directories that a shell command works in,
// as far as the command text tells them (PFM-7614): the target of each
// `cd`/`pushd`, the path of `git -C`, and the directory that `git clone` or
// `gh repo clone` creates. Relative paths resolve against cwd and the `cd`
// targets before them. A path that holds an expansion ($VAR, $(…)) is unknown
// and is skipped, and so is every relative path after an unknown `cd`.
//
// It is a heuristic, not a shell interpreter. The caller checks each
// directory, so a wrong guess costs a lookup only.
func commandCheckoutDirs(command, cwd string) []string {
	var dirs []string
	add := func(dir string) {
		if !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}

	dir := cwd
	for _, words := range splitShellCommands(command) {
		for len(words) > 0 && !words[0].dynamic && shellAssignment.MatchString(words[0].text) {
			words = words[1:]
		}
		if len(words) == 0 {
			continue
		}

		name, args := words[0].text, words[1:]
		switch {
		case name == "cd" || name == "pushd":
			// Without a usable target ("cd", "cd -", "cd $X") the directory
			// is unknown from here on.
			base := dir
			dir = ""
			if i := slices.IndexFunc(args, func(w shellWord) bool { return !strings.HasPrefix(w.text, "-") }); i >= 0 {
				if d, ok := resolveShellPath(base, args[i]); ok {
					dir = d
					add(d)
				}
			}
		case name == "git":
			for _, d := range gitCommandDirs(dir, args) {
				add(d)
			}
		case name == "gh" && len(args) >= 2 && args[0].text == "repo" && args[1].text == "clone":
			args = args[2:]
			// The words after "--" are flags for git, not operands.
			if i := slices.IndexFunc(args, func(w shellWord) bool { return w.text == "--" }); i >= 0 {
				args = args[:i]
			}
			if d, ok := cloneDir(dir, args, ghCloneValueOptions); ok {
				add(d)
			}
		}
	}

	return dirs
}

// gitCommandDirs returns the directories of one git command: the -C path,
// and for a clone the directory it creates.
func gitCommandDirs(dir string, args []shellWord) []string {
	var dirs []string

	withC := false
	i := 0
	for ; i < len(args) && strings.HasPrefix(args[i].text, "-"); i++ {
		switch a := args[i].text; {
		case a == "-C":
			if i+1 == len(args) {
				return nil
			}
			i++
			d, ok := resolveShellPath(dir, args[i])
			if !ok {
				return nil
			}
			dir, withC = d, true
		case slices.Contains(gitValueOptions, a):
			i++
		}
	}

	if withC {
		dirs = append(dirs, dir)
	}

	if i < len(args) && args[i].text == "clone" {
		if d, ok := cloneDir(dir, args[i+1:], gitCloneValueOptions); ok {
			dirs = append(dirs, d)
		}
	}

	return dirs
}

// cloneDir returns the directory that a clone creates: its second operand,
// or else the name of the repository without ".git", as git clone does.
func cloneDir(base string, args []shellWord, valueOptions []string) (string, bool) {
	var operands []shellWord
	for i := 0; i < len(args); i++ {
		switch a := args[i].text; {
		case a == "--":
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case slices.Contains(valueOptions, a):
			i++
		case strings.HasPrefix(a, "-"):
		default:
			operands = append(operands, args[i])
		}
	}

	switch len(operands) {
	case 0:
		return "", false
	case 1:
		if operands[0].dynamic {
			return "", false
		}
		name := repositoryDirName(operands[0].text)
		if name == "" {
			return "", false
		}
		return resolveShellPath(base, shellWord{text: name})
	default:
		return resolveShellPath(base, operands[1])
	}
}

// repositoryDirName returns the default clone directory for a repository
// URL, path or OWNER/REPO name: its last path element without ".git".
func repositoryDirName(repo string) string {
	name := strings.TrimRight(repo, "/")
	name = strings.TrimSuffix(name, "/.git")
	name = strings.TrimSuffix(name, ".git")
	if i := strings.LastIndexAny(name, "/:"); i >= 0 {
		name = name[i+1:]
	}

	if name == "." || name == ".." {
		return ""
	}

	return name
}

// resolveShellPath returns the absolute path that w names, from base. It
// fails for a word with an expansion, and for a relative path when base is
// unknown.
func resolveShellPath(base string, w shellWord) (string, bool) {
	if w.dynamic || w.text == "" {
		return "", false
	}

	p := w.text
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		p = filepath.Join(home, p[1:])
	}

	if filepath.IsAbs(p) {
		return filepath.Clean(p), true
	}
	if base == "" {
		return "", false
	}

	return filepath.Join(base, p), true
}

// shellWord is one word of a shell command, after quote removal.
type shellWord struct {
	text string
	// dynamic is set when the word holds an expansion ($VAR, $(…), `…`),
	// whose value is not known without running the shell.
	dynamic bool
}

// splitShellCommands splits a shell command line into its simple commands,
// each one a list of words. It knows quotes, backslash escapes, comments,
// command separators and redirections, which it drops with their target.
// It does not expand anything: commandCheckoutDirs only needs the words.
func splitShellCommands(line string) [][]shellWord {
	var (
		cmds                      [][]shellWord
		cmd                       []shellWord
		word                      strings.Builder
		inWord, dynamic, skipWord bool
	)

	endWord := func() {
		if !inWord {
			return
		}
		if skipWord {
			skipWord = false
		} else {
			cmd = append(cmd, shellWord{text: word.String(), dynamic: dynamic})
		}
		word.Reset()
		inWord, dynamic = false, false
	}
	endCommand := func() {
		endWord()
		skipWord = false
		if len(cmd) > 0 {
			cmds = append(cmds, cmd)
		}
		cmd = nil
	}

	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		switch c := runes[i]; {
		case c == ' ' || c == '\t':
			endWord()
		case strings.ContainsRune("\n;&|()", c):
			endCommand()
		case c == '>' || c == '<':
			// A file descriptor number before the operator ("2>") belongs
			// to it, not to the command.
			if inWord && isDigits(word.String()) {
				word.Reset()
				inWord, dynamic = false, false
			} else {
				endWord()
			}
			for i+1 < len(runes) && strings.ContainsRune(">&|", runes[i+1]) {
				i++
			}
			skipWord = true
		case c == '#' && !inWord:
			for i+1 < len(runes) && runes[i+1] != '\n' {
				i++
			}
		case c == '\\':
			if i+1 < len(runes) {
				i++
				if runes[i] == '\n' {
					continue
				}
				word.WriteRune(runes[i])
				inWord = true
			}
		case c == '\'':
			inWord = true
			for i+1 < len(runes) {
				i++
				if runes[i] == '\'' {
					break
				}
				word.WriteRune(runes[i])
			}
		case c == '"':
			inWord = true
			for i+1 < len(runes) {
				i++
				d := runes[i]
				if d == '"' {
					break
				}
				if d == '\\' && i+1 < len(runes) && strings.ContainsRune("\"\\$`\n", runes[i+1]) {
					i++
					word.WriteRune(runes[i])
					continue
				}
				if d == '$' || d == '`' {
					dynamic = true
				}
				word.WriteRune(d)
			}
		default:
			if c == '$' || c == '`' {
				dynamic = true
			}
			word.WriteRune(c)
			inWord = true
		}
	}
	endCommand()

	return cmds
}

func isDigits(s string) bool {
	return s != "" && strings.Trim(s, "0123456789") == ""
}
