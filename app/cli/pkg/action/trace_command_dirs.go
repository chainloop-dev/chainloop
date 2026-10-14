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
// and is skipped, and so is every relative path after an unknown `cd`. A `cd`
// inside parentheses applies only until the closing parenthesis.
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
	// outer holds the directory of each enclosing shell, one per open
	// parenthesis.
	var outer []string
	for _, cmd := range splitShellCommands(command) {
		switch cmd.subshell {
		case '(':
			outer = append(outer, dir)
		case ')':
			if n := len(outer); n > 0 {
				dir, outer = outer[n-1], outer[:n-1]
			}
		default:
			dir = applyShellCommand(cmd.words, dir, add)
		}
	}

	return dirs
}

// applyShellCommand passes to add each directory that one simple command
// names, and returns the directory that the shell is in after it.
func applyShellCommand(words []shellWord, dir string, add func(string)) string {
	// The name of an assignment is never an expansion, whatever its value.
	for len(words) > 0 && shellAssignment.MatchString(words[0].text) {
		words = words[1:]
	}
	if len(words) == 0 {
		return dir
	}

	name, args := words[0].text, words[1:]
	switch {
	case name == "cd" || name == "pushd":
		// Without a usable target ("cd -", "pushd", "cd $X") the directory
		// is unknown from here on.
		d, ok := cdTarget(dir, args, name == "cd")
		if !ok {
			return ""
		}
		add(d)
		return d
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

	return dir
}

// cdTarget returns the directory that a cd or pushd with args moves to. A cd
// without arguments moves to the home directory, when homeIfBare is set.
func cdTarget(dir string, args []shellWord, homeIfBare bool) (string, bool) {
	i := slices.IndexFunc(args, func(w shellWord) bool { return !strings.HasPrefix(w.text, "-") })
	switch {
	case i >= 0:
		return resolveShellPath(dir, args[i])
	case len(args) == 0 && homeIfBare:
		return resolveShellPath(dir, shellWord{text: "~"})
	default:
		return "", false
	}
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

// shellCommand is one simple command of a command line, or the start or end
// of a subshell.
type shellCommand struct {
	words []shellWord
	// subshell is '(' or ')' for the start or end of a subshell, which has
	// no words. It is 0 for a simple command.
	subshell rune
}

// splitShellCommands splits a shell command line into its simple commands
// and subshell boundaries. It knows quotes, backslash escapes, comments,
// command separators and redirections, which it drops with their target.
// It does not expand anything: commandCheckoutDirs only needs the words.
func splitShellCommands(line string) []shellCommand {
	l := &shellLexer{runes: []rune(line)}
	for ; l.pos < len(l.runes); l.pos++ {
		l.step(l.runes[l.pos])
	}
	l.endCommand()

	return l.cmds
}

// shellLexer holds the state of splitShellCommands.
type shellLexer struct {
	runes []rune
	pos   int
	cmds  []shellCommand
	words []shellWord
	word  strings.Builder
	// inWord is set while a word is open, also an empty quoted one.
	inWord bool
	// dynamic is set when the open word holds an expansion.
	dynamic bool
	// skipWord drops the next word, the target of a redirection.
	skipWord bool
}

func (l *shellLexer) step(c rune) {
	switch {
	case c == ' ' || c == '\t':
		l.endWord()
	case c == '(' || c == ')':
		l.endCommand()
		l.cmds = append(l.cmds, shellCommand{subshell: c})
	case strings.ContainsRune("\n;&|", c):
		l.endCommand()
	case c == '>' || c == '<':
		l.redirection()
	case c == '#' && !l.inWord:
		// A comment runs to the end of the line.
		for l.peek() != 0 && l.peek() != '\n' {
			l.pos++
		}
	case c == '\\':
		l.escape()
	case c == '\'':
		l.singleQuoted()
	case c == '"':
		l.doubleQuoted()
	default:
		l.dynamic = l.dynamic || c == '$' || c == '`'
		l.add(c)
	}
}

// peek returns the rune after the current one, or 0 at the end.
func (l *shellLexer) peek() rune {
	if l.pos+1 < len(l.runes) {
		return l.runes[l.pos+1]
	}

	return 0
}

func (l *shellLexer) add(c rune) {
	l.word.WriteRune(c)
	l.inWord = true
}

func (l *shellLexer) endWord() {
	if !l.inWord {
		return
	}
	if l.skipWord {
		l.skipWord = false
	} else {
		l.words = append(l.words, shellWord{text: l.word.String(), dynamic: l.dynamic})
	}
	l.word.Reset()
	l.inWord, l.dynamic = false, false
}

func (l *shellLexer) endCommand() {
	l.endWord()
	l.skipWord = false
	if len(l.words) > 0 {
		l.cmds = append(l.cmds, shellCommand{words: l.words})
	}
	l.words = nil
}

// redirection drops a redirection operator and marks its target for
// dropping. A file descriptor number before the operator ("2>") belongs to
// it, not to the command.
func (l *shellLexer) redirection() {
	if l.inWord && isDigits(l.word.String()) {
		l.word.Reset()
		l.inWord, l.dynamic = false, false
	} else {
		l.endWord()
	}
	for strings.ContainsRune(">&|", l.peek()) {
		l.pos++
	}
	l.skipWord = true
}

// escape adds the rune after a backslash as it is. A backslash before a
// line break joins the lines.
func (l *shellLexer) escape() {
	next := l.peek()
	if next == 0 {
		return
	}
	l.pos++
	if next != '\n' {
		l.add(next)
	}
}

func (l *shellLexer) singleQuoted() {
	l.inWord = true
	for l.peek() != 0 {
		l.pos++
		if l.runes[l.pos] == '\'' {
			return
		}
		l.word.WriteRune(l.runes[l.pos])
	}
}

func (l *shellLexer) doubleQuoted() {
	l.inWord = true
	for l.peek() != 0 {
		l.pos++
		c := l.runes[l.pos]
		switch {
		case c == '"':
			return
		case c == '\\' && strings.ContainsRune("\"\\$`\n", l.peek()):
			l.pos++
			l.word.WriteRune(l.runes[l.pos])
		default:
			l.dynamic = l.dynamic || c == '$' || c == '`'
			l.word.WriteRune(c)
		}
	}
}

func isDigits(s string) bool {
	return s != "" && strings.Trim(s, "0123456789") == ""
}
