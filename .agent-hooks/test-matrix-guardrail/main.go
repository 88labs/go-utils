// Command test-matrix-guardrail is a dependency-free PreToolUse guard.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type decision struct {
	Allow  bool
	Reason string
}
type marker struct {
	Head         string `json:"head"`
	TestDiffHash string `json:"test_diff_hash"`
	Command      string `json:"command"`
}

func main() {
	a := os.Args[1:]
	if len(a) == 0 {
		a = []string{"check"}
	}
	if a[0] == "baseline" {
		if len(a) > 1 && a[1] == "--" {
			a = a[2:]
		} else {
			a = a[1:]
		}
		if err := baseline(".", a); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if a[0] != "check" {
		fmt.Fprintln(os.Stderr, "usage: check --client <client> | baseline -- <test command>")
		os.Exit(2)
	}
	f := flag.NewFlagSet("check", flag.ContinueOnError)
	root := f.String("root", ".", "repository root")
	client := f.String("client", "codex", "hook client")
	if err := f.Parse(a[1:]); err != nil {
		os.Exit(2)
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		output(*client, decision{Reason: err.Error()})
		os.Exit(1)
	}
	d, err := check(*root, b)
	if err != nil {
		d = decision{Reason: "malformed or unsupported hook payload: " + err.Error()}
	}
	output(*client, d)
}

func check(root string, payload []byte) (decision, error) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return decision{}, err
	}
	tool := stringValue(raw, "tool_name", "toolName", "name")
	if tool == "" {
		if len(commandStrings(raw)) == 0 {
			return decision{}, errors.New("tool name is required")
		}
		tool = "shell"
	}
	in := mapValue(raw, "tool_input", "toolInput", "input")
	if in == nil {
		in = raw
	}
	if shellWrite(tool, in) {
		return decision{Reason: "shell writes are denied; use an approved file-edit tool"}, nil
	}
	if !fileEdit(tool) {
		return decision{Allow: true, Reason: "non-file-edit tool"}, nil
	}
	paths := paths(in)
	if len(paths) == 0 {
		return decision{Reason: "file-edit path cannot be extracted"}, nil
	}
	for _, p := range paths {
		if !safe(root, p) {
			return decision{Reason: "file-edit path is outside the repository"}, nil
		}
		if !allowed(repoPath(root, p)) {
			ok, err := valid(root)
			if err != nil || !ok {
				return decision{Reason: "production edit denied: baseline marker is absent or stale"}, nil
			}
		}
	}
	return decision{Allow: true, Reason: "approved file-edit path"}, nil
}

func output(client string, d decision) {
	_ = json.NewEncoder(os.Stdout).Encode(outputMap(client, d))
}
func outputMap(client string, d decision) map[string]any {
	s := "deny"
	if d.Allow {
		s = "allow"
	}
	var v map[string]any
	switch strings.ToLower(client) {
	case "claude", "codex":
		v = map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "PreToolUse", "permissionDecision": s, "permissionDecisionReason": d.Reason}}
	case "copilot":
		v = map[string]any{"permissionDecision": s, "permissionDecisionReason": d.Reason}
	case "cursor":
		v = map[string]any{"permission": s}
		if !d.Allow {
			v["user_message"] = d.Reason
			v["agent_message"] = d.Reason
		}
	default:
		v = map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "PreToolUse", "permissionDecision": s, "permissionDecisionReason": d.Reason}}
	}
	return v
}

func stringValue(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
func mapValue(m map[string]any, keys ...string) map[string]any {
	for _, k := range keys {
		if v, ok := m[k].(map[string]any); ok {
			return v
		}
	}
	return nil
}
func fileEdit(tool string) bool {
	n := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(tool))
	for _, s := range []string{"edit", "write", "patch", "createfile", "deletefile", "movefile", "replace"} {
		if strings.Contains(n, s) {
			return true
		}
	}
	return false
}
func shellWrite(tool string, m map[string]any) bool {
	n := strings.ToLower(strings.TrimSpace(tool))
	if n != "bash" && n != "sh" && n != "zsh" && n != "pwsh" && n != "powershell" && !strings.Contains(n, "shell") && !strings.Contains(n, "terminal") && !strings.Contains(n, "exec") && !strings.Contains(n, "command") && !strings.Contains(n, "run") {
		return false
	}
	c := strings.ToLower(strings.Join(commandStrings(m), " "))
	for _, s := range []string{" >", ">>", "tee ", "cat >", "sed -i", "perl -i", "rm ", "mv ", "cp ", "touch ", "mkdir ", "install ", "truncate ", "dd of=", "git apply", "git mv", "git rm", "vim ", "nano ", "emacs ", "ed "} {
		if strings.Contains(c, s) || strings.HasPrefix(c, strings.TrimSpace(s)) {
			return true
		}
	}
	return false
}
func commandStrings(m map[string]any) []string {
	var out []string
	for _, k := range []string{"command", "cmd", "script", "args", "patch"} {
		if s, ok := m[k].(string); ok {
			out = append(out, s)
		}
		if a, ok := m[k].([]any); ok {
			for _, x := range a {
				if s, ok := x.(string); ok {
					out = append(out, s)
				}
			}
		}
	}
	return out
}
func paths(m map[string]any) []string {
	var out []string
	var walk func(any, string)
	walk = func(v any, k string) {
		switch x := v.(type) {
		case map[string]any:
			for ck, cv := range x {
				walk(cv, ck)
			}
		case []any:
			for _, cv := range x {
				walk(cv, k)
			}
		case string:
			if pathKey(k) {
				out = append(out, x)
			}
		}
	}
	walk(m, "")
	for _, p := range commandStrings(m) {
		for _, line := range strings.Split(p, "\n") {
			for _, pre := range []string{"*** Update File: ", "*** Add File: ", "*** Delete File: "} {
				if strings.HasPrefix(line, pre) {
					out = append(out, strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(line, pre)), "a/"))
				}
			}
		}
	}
	sort.Strings(out)
	return unique(out)
}
func pathKey(k string) bool {
	k = strings.ToLower(strings.ReplaceAll(k, "_", ""))
	return k == "path" || k == "filepath" || k == "targetfile" || k == "oldpath" || k == "newpath" || k == "file" || k == "files"
}
func safe(root, p string) bool {
	if p == "" || strings.ContainsRune(p, 0) {
		return false
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	base = filepath.Clean(base)
	candidate := p
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(base, candidate)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(base, filepath.Clean(candidate))
	if err != nil {
		return false
	}
	return relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
func repoPath(root, p string) string {
	if !filepath.IsAbs(p) {
		return p
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return p
	}
	if relative, err := filepath.Rel(filepath.Clean(base), filepath.Clean(p)); err == nil {
		return relative
	}
	return p
}
func allowed(p string) bool {
	p = filepath.ToSlash(filepath.Clean(p))
	b := filepath.Base(p)
	if b == "AGENTS.md" || b == "CLAUDE.md" || b == "README.md" || strings.HasSuffix(b, ".instructions.md") {
		return true
	}
	if strings.HasSuffix(b, "_test.go") || strings.HasSuffix(b, ".test.go") {
		return true
	}
	for _, prefix := range []string{".claude/", ".codex/", ".cursor/", ".agent-hooks/", "docs/", "doc/", "test/", "tests/"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	if p == ".claude/settings.json" || p == ".codex/hooks.json" || p == ".cursor/hooks.json" {
		return true
	}
	if strings.HasPrefix(p, ".github/hooks/") && strings.HasSuffix(b, ".json") {
		return true
	}
	if p == ".github/copilot-instructions.md" || p == ".github/PULL_REQUEST_TEMPLATE.md" || p == ".github/pull_request_template.md" {
		return true
	}
	if strings.HasPrefix(p, ".github/agents/") && strings.HasSuffix(b, ".md") {
		return true
	}
	return strings.HasPrefix(p, ".github/instructions/") && strings.HasSuffix(b, ".md")
}
func git(root string, args ...string) (string, error) {
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	b, e := c.Output()
	return strings.TrimSpace(string(b)), e
}
func markerPath(root string) (string, error) {
	p, e := git(root, "rev-parse", "--git-path", "agent-test-first/baseline.json")
	if e != nil {
		return "", errors.New("repository metadata is unavailable")
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	return filepath.Clean(p), nil
}
func valid(root string) (bool, error) {
	p, e := markerPath(root)
	if e != nil {
		return false, e
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return false, nil
	}
	var m marker
	if json.Unmarshal(b, &m) != nil {
		return false, nil
	}
	h, e := git(root, "rev-parse", "HEAD")
	if e != nil {
		return false, e
	}
	t, e := testHash(root)
	if e != nil {
		return false, e
	}
	return m.Head == h && m.TestDiffHash == t && m.Command != "", nil
}
func baseline(root string, args []string) error {
	markerPathValue, err := markerPath(root)
	if err != nil {
		return fmt.Errorf("cannot locate baseline marker: %w", err)
	}
	if err := os.Remove(markerPathValue); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot remove baseline marker: %w", err)
	}
	if len(args) == 0 {
		return errors.New("baseline test command is required")
	}
	if !approved(root, args) {
		return errors.New("baseline command is not approved")
	}
	st, e := git(root, "status", "--porcelain=v1", "--untracked-files=all")
	if e != nil {
		return errors.New("cannot inspect repository status")
	}
	changed := false
	for _, l := range strings.Split(st, "\n") {
		p := statusPath(l)
		if p == "" {
			continue
		}
		for _, changedPath := range statusPaths(l) {
			if !allowed(changedPath) {
				return fmt.Errorf("production files are dirty: %s", changedPath)
			}
			if !isDeleted(l) && isTest(changedPath) {
				changed = true
			}
		}
	}
	if !changed {
		return errors.New("focused test addition or change is required")
	}
	// approved currently permits only the repository's exact Taskfile test command.
	// Keep the executable and arguments static so untrusted hook input cannot reach exec.Command.
	c := exec.Command("task", "test")
	c.Dir = root
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if e := c.Run(); e != nil {
		return fmt.Errorf("baseline test failed: %w", e)
	}
	h, e := git(root, "rev-parse", "HEAD")
	if e != nil {
		return e
	}
	t, e := testHash(root)
	if e != nil {
		return e
	}
	p, e := markerPath(root)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	b, _ := json.MarshalIndent(marker{Head: h, TestDiffHash: t, Command: strings.Join(args, "\x00")}, "", "  ")
	b = append(b, '\n')
	return os.WriteFile(p, b, 0600)
}
func approved(root string, a []string) bool {
	n := filepath.Base(a[0])
	if n == "task" {
		return fileExists(filepath.Join(root, "Taskfile.yaml")) && len(a) == 3 && a[1] == "-p" && a[2] == "test"
	}
	return false
}
func fileExists(p string) bool { _, e := os.Stat(p); return e == nil }
func isDeleted(l string) bool  { return len(l) >= 2 && (l[0] == 'D' || l[1] == 'D') }
func statusPath(l string) string {
	paths := statusPaths(l)
	if len(paths) == 0 {
		return ""
	}
	return paths[len(paths)-1]
}
func statusPaths(l string) []string {
	if len(l) < 4 {
		return nil
	}
	p := strings.TrimSpace(l[3:])
	if i := strings.LastIndex(p, " -> "); i >= 0 {
		return []string{strings.Trim(p[:i], "\""), strings.Trim(p[i+4:], "\"")}
	}
	return []string{strings.Trim(p, "\"")}
}
func isTest(p string) bool {
	p = filepath.ToSlash(p)
	b := filepath.Base(p)
	return strings.Contains(p, "/test/") || strings.Contains(p, "/tests/") || strings.HasSuffix(b, "_test.go") || strings.HasSuffix(b, ".test.go")
}
func testHash(root string) (string, error) {
	st, e := git(root, "status", "--porcelain=v1", "--untracked-files=all")
	if e != nil {
		return "", e
	}
	paths := []string{}
	for _, l := range strings.Split(st, "\n") {
		p := statusPath(l)
		if isTest(p) {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	d, e := exec.Command("git", append([]string{"-C", root, "diff", "--binary", "HEAD", "--"}, paths...)...).Output()
	if e != nil {
		return "", e
	}
	h := sha256.New()
	h.Write(d)
	for _, p := range unique(paths) {
		b, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if e == nil {
			h.Write([]byte(p + "\n"))
			h.Write(b)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func unique(a []string) []string {
	if len(a) < 2 {
		return a
	}
	o := a[:1]
	for _, v := range a[1:] {
		if v != o[len(o)-1] {
			o = append(o, v)
		}
	}
	return o
}
