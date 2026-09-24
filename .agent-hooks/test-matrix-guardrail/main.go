// Command test-matrix-guardrail is a dependency-free PreToolUse guard.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
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
		rel := repoPath(root, p)
		if allowed(rel) || !isGoPath(rel) || isTest(rel) {
			continue
		}
		needsBaseline, err := functionChangeRequiresBaseline(root, tool, in, rel)
		if err != nil {
			return decision{}, err
		}
		if !needsBaseline {
			continue
		}
		ok, err := valid(root)
		if err != nil || !ok {
			return decision{Reason: "production edit denied: baseline marker is absent or stale"}, nil
		}
	}
	return decision{Allow: true, Reason: "approved file-edit path"}, nil
}

type patchSection struct {
	action string
	path   string
	lines  []string
}

func functionChangeRequiresBaseline(root, tool string, input map[string]any, path string) (bool, error) {
	patch, ok, err := patchPayload(input)
	if err != nil {
		return false, err
	}
	if ok {
		sections := patchHeaders(patch)
		if len(sections) == 0 {
			return false, errors.New("patch contains no valid file headers")
		}
		for _, section := range sections {
			rel := repoPath(root, section.path)
			if !safe(root, rel) {
				return false, errors.New("patch path is outside the repository")
			}
			if allowed(rel) || !isGoPath(rel) || !patchSectionChangesContent(section) {
				continue
			}
			current, err := readGoSource(root, rel, section.action == "Add")
			if err != nil {
				return false, err
			}
			candidate, err := applyPatchSection(current, section)
			if err != nil {
				return false, fmt.Errorf("cannot inspect Go patch for %s: %w", rel, err)
			}
			changed, err := functionDefinitionsChanged(current, candidate)
			if err != nil {
				return false, fmt.Errorf("cannot inspect Go patch for %s: %w", rel, err)
			}
			if changed {
				return true, nil
			}
		}
		return false, nil
	}

	toolName := normalizedToolName(tool)
	if toolName == "delete" || strings.Contains(toolName, "deletefile") {
		current, err := readGoSource(root, path, true)
		if err != nil {
			return false, err
		}
		functions, err := functionDefinitions(current)
		if err != nil {
			return false, fmt.Errorf("cannot inspect Go file %s: %w", path, err)
		}
		return len(functions) > 0, nil
	}

	content, hasContent, err := inputString(input, "content", "new_content")
	if err != nil {
		return false, err
	}
	oldString, hasOld, err := inputString(input, "old_string", "oldString")
	if err != nil {
		return false, err
	}
	newString, hasNew, err := inputString(input, "new_string", "newString")
	if err != nil {
		return false, err
	}
	if hasOld != hasNew {
		return false, errors.New("old and new edit strings must be provided together")
	}
	if !hasContent && !hasOld {
		return false, nil
	}

	current, err := readGoSource(root, path, hasContent)
	if err != nil {
		return false, err
	}
	candidate := content
	if hasOld {
		if oldString == "" || strings.Count(current, oldString) != 1 {
			return false, fmt.Errorf("old edit string is not uniquely found in %s", path)
		}
		candidate = strings.Replace(current, oldString, newString, 1)
	}
	return functionDefinitionsChanged(current, candidate)
}

func patchPayload(input map[string]any) (string, bool, error) {
	for _, key := range []string{"patch", "apply_patch", "command"} {
		value, ok := input[key]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return "", false, fmt.Errorf("%s is not a string", key)
		}
		if key == "command" && !strings.Contains(text, "*** ") {
			continue
		}
		return text, true, nil
	}
	return "", false, nil
}

func inputString(input map[string]any, keys ...string) (string, bool, error) {
	for _, key := range keys {
		value, ok := input[key]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return "", false, fmt.Errorf("%s is not a string", key)
		}
		return text, true, nil
	}
	return "", false, nil
}

func patchHeaders(patch string) []patchSection {
	lines := strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n")
	sections := make([]patchSection, 0)
	for i := 0; i < len(lines); i++ {
		action, path, ok := patchHeader(lines[i])
		if !ok {
			continue
		}
		section := patchSection{action: action, path: path}
		for i++; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "*** End Patch" {
				break
			}
			if _, _, nextOK := patchHeader(lines[i]); nextOK {
				i--
				break
			}
			section.lines = append(section.lines, lines[i])
		}
		sections = append(sections, section)
	}
	return sections
}

func patchHeader(line string) (string, string, bool) {
	for _, action := range []string{"Update", "Add", "Delete"} {
		prefix := "*** " + action + " File:"
		if strings.HasPrefix(line, prefix) {
			path := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			return action, path, path != ""
		}
	}
	return "", "", false
}

func patchSectionChangesContent(section patchSection) bool {
	if section.action == "Delete" || section.action == "Add" {
		return true
	}
	for _, line := range section.lines {
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			return true
		}
	}
	return false
}

func isGoPath(path string) bool {
	return strings.HasSuffix(strings.ToLower(filepath.ToSlash(path)), ".go")
}

func readGoSource(root, path string, allowMissing bool) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if errors.Is(err, os.ErrNotExist) && allowMissing {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", path, err)
	}
	return string(b), nil
}

func applyPatchSection(current string, section patchSection) (string, error) {
	switch section.action {
	case "Add":
		lines := make([]string, 0, len(section.lines))
		for _, line := range section.lines {
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "+") {
				return "", errors.New("added Go patch line is missing '+' prefix")
			}
			lines = append(lines, line[1:])
		}
		return strings.Join(lines, "\n") + "\n", nil
	case "Delete":
		return "", nil
	case "Update":
		return applyUpdatePatch(current, section.lines)
	default:
		return "", fmt.Errorf("unsupported patch action %q", section.action)
	}
}

func applyUpdatePatch(current string, patchLines []string) (string, error) {
	currentLines, trailingNewline := sourceLines(current)
	result := make([]string, 0, len(currentLines))
	cursor := 0
	hunk := make([]string, 0)
	flushHunk := func() error {
		if len(hunk) == 0 {
			return nil
		}
		oldLines := make([]string, 0, len(hunk))
		for _, line := range hunk {
			if line[0] == ' ' || line[0] == '-' {
				oldLines = append(oldLines, line[1:])
			}
		}
		start := cursor
		if len(oldLines) > 0 {
			start = findLines(currentLines, oldLines, cursor)
			if start < 0 {
				return errors.New("patch context does not match current Go source")
			}
		}
		result = append(result, currentLines[cursor:start]...)
		for _, line := range hunk {
			switch line[0] {
			case ' ':
				result = append(result, line[1:])
			case '+':
				result = append(result, line[1:])
			}
		}
		if len(oldLines) > 0 {
			cursor = start + len(oldLines)
		}
		hunk = nil
		return nil
	}

	for _, line := range patchLines {
		if strings.HasPrefix(line, "@@") {
			if err := flushHunk(); err != nil {
				return "", err
			}
			continue
		}
		if line == "" || line == `\ No newline at end of file` {
			continue
		}
		if line[0] != ' ' && line[0] != '+' && line[0] != '-' {
			return "", errors.New("updated Go patch line has an invalid prefix")
		}
		hunk = append(hunk, line)
	}
	if err := flushHunk(); err != nil {
		return "", err
	}
	result = append(result, currentLines[cursor:]...)
	return joinSourceLines(result, trailingNewline), nil
}

func sourceLines(source string) ([]string, bool) {
	trailingNewline := strings.HasSuffix(source, "\n")
	if trailingNewline {
		source = strings.TrimSuffix(source, "\n")
	}
	if source == "" {
		return nil, trailingNewline
	}
	return strings.Split(source, "\n"), trailingNewline
}

func joinSourceLines(lines []string, trailingNewline bool) string {
	result := strings.Join(lines, "\n")
	if trailingNewline {
		result += "\n"
	}
	return result
}

func findLines(source, target []string, from int) int {
	for start := from; start+len(target) <= len(source); start++ {
		matched := true
		for i := range target {
			if source[start+i] != target[i] {
				matched = false
				break
			}
		}
		if matched {
			return start
		}
	}
	return -1
}

func functionDefinitionsChanged(before, after string) (bool, error) {
	beforeFunctions, err := functionDefinitions(before)
	if err != nil {
		return false, err
	}
	afterFunctions, err := functionDefinitions(after)
	if err != nil {
		return false, err
	}
	if len(beforeFunctions) != len(afterFunctions) {
		return true, nil
	}
	for name, beforeDefinition := range beforeFunctions {
		if afterFunctions[name] != beforeDefinition {
			return true, nil
		}
	}
	return false, nil
}

func functionDefinitions(source string) (map[string]string, error) {
	definitions := make(map[string]string)
	if strings.TrimSpace(source) == "" {
		return definitions, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "edited.go", source, 0)
	if err != nil {
		return nil, err
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		var receiver bytes.Buffer
		if function.Recv != nil {
			if err := format.Node(&receiver, fset, function.Recv); err != nil {
				return nil, err
			}
		}
		var definition bytes.Buffer
		if err := format.Node(&definition, fset, function); err != nil {
			return nil, err
		}
		name := function.Name.Name
		if receiver.Len() > 0 {
			name = receiver.String() + "." + name
		}
		definitions[name] = definition.String()
	}
	return definitions, nil
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
	n := normalizedToolName(tool)
	for _, s := range []string{"edit", "write", "patch", "createfile", "deletefile", "delete", "movefile", "replace"} {
		if strings.Contains(n, s) {
			return true
		}
	}
	return false
}
func normalizedToolName(tool string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(tool))
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
			for _, pre := range []string{"*** Update File: ", "*** Add File: ", "*** Delete File: ", "*** Move to: "} {
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
