package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repo(t *testing.T) string {
	t.Helper()
	r := t.TempDir()
	if e := os.WriteFile(filepath.Join(r, "go.mod"), []byte("module example.com/g\n\ngo 1.23\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(r, "Taskfile.yaml"), []byte("version: '3'\n\ntasks:\n  test:\n    cmds:\n      - go test ./...\n"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, a := range [][]string{{"init", "-q"}, {"add", "go.mod", "Taskfile.yaml"}, {"-c", "user.email=t@e", "-c", "user.name=t", "commit", "-qm", "base"}} {
		if o, e := exec.Command("git", append([]string{"-C", r}, a...)...).CombinedOutput(); e != nil {
			t.Fatalf("git: %v %s", e, o)
		}
	}
	return r
}
func TestClientsAllowDenyAndOutputs(t *testing.T) {
	r := repo(t)
	for _, c := range []string{"claude", "codex", "copilot", "cursor"} {
		a, e := check(r, []byte(`{"tool_name":"Read","tool_input":{"path":"service.go"}}`))
		if e != nil || !a.Allow {
			t.Fatalf("%s read: %#v %v", c, a, e)
		}
		d, _ := check(r, []byte(`{"toolName":"Edit","toolInput":{"filePath":"service.go"}}`))
		if !d.Allow {
			t.Fatalf("%s path-only Go edit should not require a baseline: %#v", c, d)
		}
		h, e := check(r, []byte(`{"tool_name":"Edit","tool_input":{"filePath":".github/hooks/pre-tool-use.json"}}`))
		if e != nil || !h.Allow {
			t.Fatalf("%s hook config: %#v %v", c, h, e)
		}
		w, e := check(r, []byte(`{"tool_name":"Edit","tool_input":{"filePath":".github/workflows/deploy.yml"}}`))
		if e != nil || !w.Allow {
			t.Fatalf("%s workflow config should not require a Go baseline: %#v %v", c, w, e)
		}
		out := outputMap(c, decision{Reason: "deny"})
		if _, e := json.Marshal(out); e != nil {
			t.Fatal(e)
		}
		switch c {
		case "claude", "codex":
			h := out["hookSpecificOutput"].(map[string]any)
			if h["hookEventName"] != "PreToolUse" || h["permissionDecision"] != "deny" || h["permissionDecisionReason"] != "deny" {
				t.Fatalf("%s output: %#v", c, out)
			}
		case "copilot":
			if out["permissionDecision"] != "deny" || out["permissionDecisionReason"] != "deny" {
				t.Fatalf("copilot output: %#v", out)
			}
		case "cursor":
			if out["permission"] != "deny" || out["user_message"] == nil || out["agent_message"] == nil {
				t.Fatalf("cursor output: %#v", out)
			}
		}
	}
}

func TestCurrentGuardRejectsProductionGoFunctionEdits(t *testing.T) {
	r := repo(t)
	if err := os.WriteFile(filepath.Join(r, "service.go"), []byte("package g\n\nfunc Value() int { return 1 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"tool_name":"Edit","tool_input":{"filePath":"service.go","old_string":"return 1","new_string":"return 2"}}`)
	d, _ := check(r, payload)
	if d.Allow {
		t.Fatalf("production Go function edit must require a baseline: %#v", d)
	}
}

func TestCheckAllowsNonFunctionGoAndNonGoEdits(t *testing.T) {
	const current = `package g

const version = 1

func Value() int {
	return version
}
`
	for _, tc := range []struct {
		name, path, oldValue, newValue, content string
	}{
		{name: "non-function Go edit", path: "service.go", oldValue: "const version = 1", newValue: "const version = 2"},
		{name: "comment-only Go edit", path: "service.go", oldValue: "func Value() int {\n\treturn version\n}", newValue: "// Value returns the version.\nfunc Value() int {\n\treturn version\n}"},
		{name: "test Go edit", path: "service_test.go", content: "package g\n\nfunc TestValue(t *testing.T) { t.Fatal() }\n"},
		{name: "non-Go edit", path: "config.yaml", content: "enabled: true\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := repo(t)
			if err := os.WriteFile(filepath.Join(r, "service.go"), []byte(current), 0600); err != nil {
				t.Fatal(err)
			}
			var payload string
			if tc.content != "" {
				payload = `{"tool_name":"Edit","tool_input":{"filePath":"` + tc.path + `","content":` + mustJSON(tc.content) + `}}`
			} else {
				payload = `{"tool_name":"Edit","tool_input":{"filePath":"` + tc.path + `","old_string":` + mustJSON(tc.oldValue) + `,"new_string":` + mustJSON(tc.newValue) + `}}`
			}
			d, err := check(r, []byte(payload))
			if err != nil || !d.Allow {
				t.Fatalf("got %#v %v", d, err)
			}
		})
	}
}

func TestCheckAllowsNonFunctionPatchAndGatesFunctionPatch(t *testing.T) {
	r := repo(t)
	if err := os.MkdirAll(filepath.Join(r, "lib"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r, "lib", "production.go"), []byte("package production\n\nconst version = 1\n\nfunc Value() int {\n\treturn version\n}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	nonFunctionPatch := "*** Begin Patch\n*** Update File: lib/production.go\n@@\n-const version = 1\n+const version = 2\n*** End Patch\n"
	d, err := check(r, []byte(`{"tool_name":"apply_patch","tool_input":{"command":`+mustJSON(nonFunctionPatch)+`}}`))
	if err != nil || !d.Allow {
		t.Fatalf("non-function patch got %#v %v", d, err)
	}
	functionPatch := "*** Begin Patch\n*** Update File: lib/production.go\n@@\n func Value() int {\n-\treturn version\n+\treturn version + 1\n }\n*** End Patch\n"
	d, err = check(r, []byte(`{"tool_name":"apply_patch","tool_input":{"command":`+mustJSON(functionPatch)+`}}`))
	if err != nil || d.Allow {
		t.Fatalf("function patch should require a baseline: %#v %v", d, err)
	}
}

func TestCheckRejectsPatchMoveOutsideRepository(t *testing.T) {
	r := repo(t)
	for _, source := range []string{"safe.go", "README.md"} {
		patch := "*** Begin Patch\n*** Update File: " + source + "\n*** Move to: ../outside.go\n*** End Patch\n"
		payload := []byte(`{"tool_name":"apply_patch","tool_input":{"command":` + mustJSON(patch) + `}}`)
		d, err := check(r, payload)
		if err != nil || d.Allow || !strings.Contains(d.Reason, "outside the repository") {
			t.Fatalf("move from %s outside repository: %#v %v", source, d, err)
		}
	}
	patch := "*** Begin Patch\n*** Update File: safe.go\n*** Move to: renamed.go\n*** End Patch\n"
	payload := []byte(`{"tool_name":"apply_patch","tool_input":{"command":` + mustJSON(patch) + `}}`)
	if d, err := check(r, payload); err != nil || !d.Allow {
		t.Fatalf("move within repository: %#v %v", d, err)
	}
}

func TestCheckAllowsMovedGoEditAfterBaseline(t *testing.T) {
	r := repo(t)
	if err := os.WriteFile(filepath.Join(r, "service.go"), []byte("package g\n\nfunc Value() int {\n\treturn 1\n}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitCommit(t, r, "service.go")
	patch := "*** Begin Patch\n*** Update File: service.go\n*** Move to: moved.go\n@@\n func Value() int {\n-\treturn 1\n+\treturn 2\n }\n*** End Patch\n"
	payload := []byte(`{"tool_name":"apply_patch","tool_input":{"command":` + mustJSON(patch) + `}}`)
	if d, err := check(r, payload); err != nil || d.Allow || !strings.Contains(d.Reason, "baseline marker") {
		t.Fatalf("moved function edit without baseline: %#v %v", d, err)
	}
	if err := os.WriteFile(filepath.Join(r, "service_test.go"), []byte("package g\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := baseline(r, []string{"task", "-p", "test"}); err != nil {
		t.Fatal(err)
	}
	if d, err := check(r, payload); err != nil || !d.Allow {
		t.Fatalf("moved function edit after baseline: %#v %v", d, err)
	}
}

func TestCheckRequiresBaselineForGoModuleBoundaryMove(t *testing.T) {
	r := repo(t)
	if err := os.WriteFile(filepath.Join(r, "service.txt"), []byte("package g\n\nfunc Value() int { return 1 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, module := range []string{"backoff", "jitter"} {
		if err := os.MkdirAll(filepath.Join(r, module), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(r, "backoff", "service.go"), []byte("package backoff\n\nfunc Value() int { return 1 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ source, destination string }{
		{source: "service.txt", destination: "service.go"},
		{source: "backoff/service.go", destination: "jitter/service.go"},
		{source: "backoff/service.go", destination: "backoff/sub/service.go"},
	} {
		patch := "*** Begin Patch\n*** Update File: " + tc.source + "\n*** Move to: " + tc.destination + "\n*** End Patch\n"
		payload := []byte(`{"tool_name":"apply_patch","tool_input":{"command":` + mustJSON(patch) + `}}`)
		if d, err := check(r, payload); err != nil || d.Allow || !strings.Contains(d.Reason, "baseline marker") {
			t.Fatalf("move %s to %s: %#v %v", tc.source, tc.destination, d, err)
		}
	}
}

func TestCheckMoveFileGatesPackageBoundary(t *testing.T) {
	r := repo(t)
	if err := os.MkdirAll(filepath.Join(r, "backoff"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r, "backoff", "service.go"), []byte("package backoff\n\nfunc Value() int { return 1 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		tool, destination string
		allow             bool
	}{
		{tool: "MoveFile", destination: "backoff/renamed.go", allow: true},
		{tool: "MoveFile", destination: "backoff/sub/service.go", allow: false},
		{tool: "move_file", destination: "jitter/service.go", allow: false},
	} {
		payload := []byte(`{"tool_name":"` + tc.tool + `","tool_input":{"oldPath":"backoff/service.go","newPath":"` + tc.destination + `"}}`)
		d, err := check(r, payload)
		if err != nil || d.Allow != tc.allow {
			t.Fatalf("%s to %s: %#v %v", tc.tool, tc.destination, d, err)
		}
	}
	if d, err := check(r, []byte(`{"tool_name":"MoveFile","tool_input":{"sourcePath":"backoff/service.go","targetPath":"backoff/renamed.go"}}`)); err != nil || !d.Allow {
		t.Fatalf("move with source and target path keys: %#v %v", d, err)
	}
}

func TestCheckAppliesGoPatchAtItsContext(t *testing.T) {
	r := repo(t)
	source := "package g\n\nconst example = `\n\tvar n = 1\n`\n\nfunc Value() int {\n\tvar n = 1\n\treturn n\n}\n"
	if err := os.WriteFile(filepath.Join(r, "service.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	patch := "*** Begin Patch\n*** Update File: service.go\n@@ func Value() int {\n-\tvar n = 1\n+\tvar n = 2\n*** End Patch\n"
	payload := []byte(`{"tool_name":"apply_patch","tool_input":{"command":` + mustJSON(patch) + `}}`)
	if d, err := check(r, payload); err != nil || d.Allow || !strings.Contains(d.Reason, "baseline marker") {
		t.Fatalf("function-context patch: %#v %v", d, err)
	}
}

func TestCheckAcceptsEndOfFilePatchMarker(t *testing.T) {
	r := repo(t)
	if err := os.WriteFile(filepath.Join(r, "service.go"), []byte("package g\n\nfunc Value() int {\n\treturn 1\n}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	patch := "*** Begin Patch\n*** Update File: service.go\n@@ func Value() int {\n-\treturn 1\n+\treturn 2\n*** End of File\n*** End Patch\n"
	payload := []byte(`{"tool_name":"apply_patch","tool_input":{"command":` + mustJSON(patch) + `}}`)
	if d, err := check(r, payload); err != nil || d.Allow || !strings.Contains(d.Reason, "baseline marker") {
		t.Fatalf("end-of-file patch: %#v %v", d, err)
	}
}

func TestCheckRequiresBaselineForDeleteFileToolNames(t *testing.T) {
	r := repo(t)
	if err := os.WriteFile(filepath.Join(r, "service.go"), []byte("package g\n\nfunc Value() int { return 1 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"DeleteFile", "delete_file", "delete-file", "delete"} {
		payload := []byte(`{"tool_name":"` + tool + `","tool_input":{"filePath":"service.go"}}`)
		d, err := check(r, payload)
		if err != nil || d.Allow || !strings.Contains(d.Reason, "baseline marker") {
			t.Fatalf("%s deleted a Go function without baseline: %#v %v", tool, d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(r, "constants.go"), []byte("package g\n\nconst Value = 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if d, err := check(r, []byte(`{"tool_name":"DeleteFile","tool_input":{"filePath":"constants.go"}}`)); err != nil || !d.Allow {
		t.Fatalf("deleting a Go file without functions: %#v %v", d, err)
	}
}

func TestMalformedShellAndPatchFailClosed(t *testing.T) {
	if _, e := check(t.TempDir(), []byte("{")); e == nil {
		t.Fatal("malformed accepted")
	}
	d, _ := check(t.TempDir(), []byte(`{"tool_name":"shell","tool_input":{"command":"echo x > app.go"}}`))
	if d.Allow {
		t.Fatal("shell write allowed")
	}
	for _, command := range []string{"cat > app.go", "tee app.go", "sed -i s/a/b/ app.go", "rm app.go", "git apply change.patch", "vim app.go"} {
		d, _ := check(t.TempDir(), []byte(`{"tool_name":"shell","tool_input":{"command":"`+command+`"}}`))
		if d.Allow {
			t.Fatalf("shell write command allowed: %s", command)
		}
	}
	for _, tool := range []string{"Bash", "sh", "zsh", "pwsh", "powershell"} {
		for _, command := range []string{"echo x > app.go", "rm app.go"} {
			d, err := check(t.TempDir(), []byte(`{"tool_name":"`+tool+`","tool_input":{"command":"`+command+`"}}`))
			if err != nil || d.Allow {
				t.Fatalf("%s write command allowed: %#v %v", tool, d, err)
			}
		}
		if d, err := check(t.TempDir(), []byte(`{"command":"ls app.go","cwd":"."}`)); err != nil || !d.Allow {
			t.Fatalf("%s read-only root shell must allow: %#v %v", tool, d, err)
		}
	}
	r := repo(t)
	d, _ = check(r, []byte(`{"tool_name":"apply_patch","tool_input":{"patch":"*** Update File: ../outside.go"}}`))
	if d.Allow {
		t.Fatal("outside patch allowed")
	}
	d, _ = check(r, []byte(`{"tool_name":"apply_patch","tool_input":{"command":"*** Update File: test/new_test.go\n@@\n+package g\n*** Update File: lib/production.go\n@@\n+func Value() int {\n+\treturn 1\n+}\n"}}`))
	if d.Allow {
		t.Fatal("mixed test and production patch allowed")
	}
}
func TestBaselineDirtyFailureSuccessAndStale(t *testing.T) {
	r := repo(t)
	if e := baseline(r, []string{"task", "-p", "test"}); e == nil {
		t.Fatal("no focused test accepted")
	}
	if e := os.WriteFile(filepath.Join(r, "service.go"), []byte("package g\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := baseline(r, []string{"task", "-p", "test"}); e == nil || !strings.Contains(e.Error(), "production") {
		t.Fatal(e)
	}
	os.Remove(filepath.Join(r, "service.go"))
	if e := os.WriteFile(filepath.Join(r, "service_test.go"), []byte("package g\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(r, "Taskfile.yaml"), []byte("version: '3'\n\ntasks:\n  test:\n    cmds:\n      - false\n"), 0600); e != nil {
		t.Fatal(e)
	}
	marker, e := markerPath(r)
	if e != nil {
		t.Fatal(e)
	}
	if e := os.MkdirAll(filepath.Dir(marker), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(marker, []byte(`{"head":"stale"}`), 0600); e != nil {
		t.Fatal(e)
	}
	if e := baseline(r, []string{"task", "-p", "test"}); e == nil {
		t.Fatal("failed test accepted")
	}
	if _, e := os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("failure wrote marker")
	}
	if e := os.WriteFile(filepath.Join(r, "Taskfile.yaml"), []byte("version: '3'\n\ntasks:\n  test:\n    cmds:\n      - go test ./...\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := baseline(r, []string{"task", "-p", "test"}); e != nil {
		t.Fatal(e)
	}
	if ok, e := valid(r, "service.go"); e != nil || !ok {
		t.Fatalf("marker invalid: %v %v", ok, e)
	}
	if e := os.WriteFile(filepath.Join(r, "service_test.go"), []byte("package g\n// stale\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if ok, _ := valid(r, "service.go"); ok {
		t.Fatal("stale marker valid")
	}
}

func TestBaselineCanUseTheEditedModuleTask(t *testing.T) {
	r := repo(t)
	taskfile := "version: '3'\n\ntasks:\n  test:\n    cmds:\n      - go test ./...\n  test-backoff:\n    dir: backoff\n    cmds:\n      - go test ./...\n  test-aws:\n    dir: aws\n    cmds:\n      - go test ./...\n"
	if err := os.WriteFile(filepath.Join(r, "Taskfile.yaml"), []byte(taskfile), 0600); err != nil {
		t.Fatal(err)
	}
	for _, module := range []string{"backoff", "aws"} {
		if err := os.MkdirAll(filepath.Join(r, module), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(r, module, "go.mod"), []byte("module example.com/"+module+"\n\ngo 1.23\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(r, module, "service.go"), []byte("package "+module+"\n\nfunc Value() int { return 1 }\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	gitCommit(t, r, ".")
	if err := os.WriteFile(filepath.Join(r, "backoff", "focus_test.go"), []byte("package backoff\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := baseline(r, []string{"task", "-p", "test-aws"}); err == nil || !strings.Contains(err.Error(), "focused test") {
		t.Fatalf("unrelated module baseline accepted: %v", err)
	}
	if err := baseline(r, []string{"task", "-p", "test-backoff"}); err != nil {
		t.Fatalf("focused module baseline failed: %v", err)
	}
	for _, tc := range []struct {
		path  string
		allow bool
	}{
		{path: "backoff/service.go", allow: true},
		{path: "aws/service.go", allow: false},
		{path: "backoff/../aws/service.go", allow: false},
	} {
		payload := []byte(`{"tool_name":"Edit","tool_input":{"filePath":"` + tc.path + `","old_string":"return 1","new_string":"return 2"}}`)
		d, err := check(r, payload)
		if err != nil || d.Allow != tc.allow {
			t.Fatalf("%s with backoff baseline: %#v %v", tc.path, d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(r, "backoff", "go.mod"), []byte("module example.com/backoff\n\ngo 1.24\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if ok, err := valid(r, "backoff/service.go"); err != nil || ok {
		t.Fatalf("marker survived module manifest change: %v %v", ok, err)
	}
}

func TestBaselineInvalidAfterTaskfileChange(t *testing.T) {
	r := repo(t)
	if err := os.WriteFile(filepath.Join(r, "service_test.go"), []byte("package g\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := baseline(r, []string{"task", "-p", "test"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r, "Taskfile.yaml"), []byte("version: '3'\n\ntasks:\n  test:\n    cmds:\n      - go test -run ^$ ./...\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if ok, err := valid(r, "service.go"); err != nil || ok {
		t.Fatalf("marker survived test task change: %v %v", ok, err)
	}
}

func TestApprovedExistingUnderscoreModuleTask(t *testing.T) {
	if !approved("../..", []string{"task", "-p", "test-tspb_cast"}) {
		t.Fatal("existing tspb_cast module task should be approved")
	}
}

func TestAbsolutePathsAndDenyByDefault(t *testing.T) {
	r := repo(t)
	for _, path := range []string{"package.json", "Taskfile.yaml", "config/app.yaml", "terraform/main.tf", "scripts/release.sh", ".github/dependabot.yml"} {
		d, _ := check(r, []byte(`{"tool_name":"Edit","tool_input":{"filePath":"`+path+`"}}`))
		if !d.Allow {
			t.Fatalf("non-Go path should not require a Go baseline: %s: %#v", path, d)
		}
	}
	focus := filepath.Join(r, "service_test.go")
	if err := os.WriteFile(focus, []byte("package g\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := baseline(r, []string{"task", "-p", "test"}); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(r, "service.go")
	outside := filepath.Join(filepath.Dir(r), "outside.go")
	if d, _ := check(r, []byte(`{"tool_name":"Edit","tool_input":{"filePath":"`+inside+`"}}`)); !d.Allow {
		t.Fatalf("absolute path inside repository must allow: %#v", d)
	}
	if d, _ := check(r, []byte(`{"tool_name":"Edit","tool_input":{"filePath":"`+outside+`"}}`)); d.Allow {
		t.Fatalf("absolute path outside repository must deny: %#v", d)
	}
}

func TestBaselineRejectsProductionDeletionAndRenames(t *testing.T) {
	for _, name := range []string{"deletion", "production-to-test", "test-to-production"} {
		t.Run(name, func(t *testing.T) {
			r := repo(t)
			if name == "test-to-production" {
				if err := os.WriteFile(filepath.Join(r, "service_test.go"), []byte("package g\n"), 0600); err != nil {
					t.Fatal(err)
				}
				gitCommit(t, r, "service_test.go")
				if err := os.Rename(filepath.Join(r, "service_test.go"), filepath.Join(r, "service.go")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(filepath.Join(r, "service.go"), []byte("package g\n"), 0600); err != nil {
					t.Fatal(err)
				}
				gitCommit(t, r, "service.go")
				if name == "deletion" {
					if err := os.Remove(filepath.Join(r, "service.go")); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Rename(filepath.Join(r, "service.go"), filepath.Join(r, "service_test.go")); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(r, "focus_test.go"), []byte("package g\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := baseline(r, []string{"task", "-p", "test"}); err == nil || !strings.Contains(err.Error(), "production") {
				t.Fatalf("production %s must be rejected: %v", name, err)
			}
		})
	}
}

func TestBaselineRejectsDeletedOnlyAndMarkerRemovalFailure(t *testing.T) {
	r := repo(t)
	if err := os.WriteFile(filepath.Join(r, "service_test.go"), []byte("package g\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitCommit(t, r, "service_test.go")
	if err := os.Remove(filepath.Join(r, "service_test.go")); err != nil {
		t.Fatal(err)
	}
	if err := baseline(r, []string{"task", "-p", "test"}); err == nil || !strings.Contains(err.Error(), "focused test") {
		t.Fatalf("deleted-only test must be rejected: %v", err)
	}
	r = repo(t)
	if err := os.WriteFile(filepath.Join(r, "focus_test.go"), []byte("package g\n"), 0600); err != nil {
		t.Fatal(err)
	}
	marker, err := markerPath(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(marker, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(marker, "held"), []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := baseline(r, []string{"task", "-p", "test"}); err == nil || !strings.Contains(err.Error(), "remove baseline marker") {
		t.Fatalf("marker removal failure must abort: %v", err)
	}
}

func TestHookConfigSchemasAndCommands(t *testing.T) {
	for _, path := range []string{"../../.claude/settings.json", "../../.codex/hooks.json", "../../.github/hooks/pre-tool-use.json", "../../.cursor/hooks.json"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !strings.Contains(string(data), "ApplyPatch") || !strings.Contains(string(data), "apply_patch") || !strings.Contains(string(data), "Shell") || (strings.Contains(path, ".cursor/") && (!strings.Contains(string(data), "beforeShellExecution") || !strings.Contains(string(data), "timeout"))) {
			t.Fatalf("%s lacks cross-client shell/apply-patch coverage", path)
		}
		if strings.Contains(path, ".codex/") && !strings.Contains(string(data), "git rev-parse --show-toplevel") {
			t.Fatalf("%s must resolve repository root", path)
		}
	}
}

func TestHookMatchersCoverFileMovesAndDeletes(t *testing.T) {
	for _, tc := range []struct{ path, event string }{
		{path: "../../.claude/settings.json", event: "PreToolUse"},
		{path: "../../.codex/hooks.json", event: "PreToolUse"},
		{path: "../../.github/hooks/pre-tool-use.json", event: "PreToolUse"},
		{path: "../../.cursor/hooks.json", event: "preToolUse"},
	} {
		data, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		var config struct {
			Hooks map[string][]struct {
				Matcher string `json:"matcher"`
			} `json:"hooks"`
		}
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatal(err)
		}
		matchers := map[string]bool{}
		for _, entry := range config.Hooks[tc.event] {
			for _, name := range strings.Split(entry.Matcher, "|") {
				matchers[name] = true
			}
		}
		for _, name := range []string{"Delete", "DeleteFile", "delete_file", "Move", "MoveFile", "move_file"} {
			if !matchers[name] {
				t.Errorf("%s does not match %s", tc.path, name)
			}
		}
	}
}

func TestAgentEntrypointsRemainRepositoryLocal(t *testing.T) {
	for _, path := range []string{"../../AGENTS.md", "../../CLAUDE.md", "../../.github/copilot-instructions.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if strings.Contains(text, ".agents") || strings.Contains(text, "SKILL.md") || strings.Contains(text, "git submodule update") {
			t.Fatalf("%s must remain independent of external agent submodules", path)
		}
		if !strings.Contains(text, "test") || !strings.Contains(text, "baseline") {
			t.Fatalf("%s must retain repository-local test-matrix guidance", path)
		}
	}
}

func TestBaselineRejectsUndocumentedTaskInvocation(t *testing.T) {
	r := repo(t)
	if err := os.WriteFile(filepath.Join(r, "focus_test.go"), []byte("package g\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := baseline(r, []string{"task", "test"}); err == nil || !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("task test must be rejected when docs require task -p test: %v", err)
	}
}

func gitCommit(t *testing.T, root string, path string) {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "add", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v (%s)", err, output)
	}
	cmd = exec.Command("git", "-C", root, "-c", "user.email=test@example.com", "-c", "user.name=test", "commit", "-qm", "change")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v (%s)", err, output)
	}
}

func mustJSON(value string) string {
	b, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(b)
}
