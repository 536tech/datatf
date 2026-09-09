package cli

import (
	"context"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/536tech/datatf/internal/fakews"
)

func captureUpdateCheck(t *testing.T) *int {
	t.Helper()
	oldCheck, oldDir := checkForUpdate, updateCacheDir
	oldTerminal, oldVersion := terminalOutput, version
	t.Cleanup(func() {
		checkForUpdate, updateCacheDir = oldCheck, oldDir
		terminalOutput, version = oldTerminal, oldVersion
	})
	for _, key := range []string{
		"DATATF_NO_UPDATE_NOTIFIER", "DO_NOT_TRACK", "CI", "GITHUB_ACTIONS", "TF_BUILD",
		"GITLAB_CI", "JENKINS_URL", "CODEX_THREAD_ID", "CODEX_CI", "CLAUDECODE",
		"CLAUDE_CODE_ENTRYPOINT",
	} {
		t.Setenv(key, "")
	}
	dir := t.TempDir()
	updateCacheDir = func() (string, error) { return dir, nil }
	terminalOutput = func(io.Writer) bool { return true }
	version = "0.2.0"
	calls := 0
	checkForUpdate = func(context.Context, string, string, time.Time) string {
		calls++
		return "v0.3.0"
	}
	return &calls
}

func TestUpdateNoticeForInteractiveLaunch(t *testing.T) {
	calls := captureUpdateCheck(t)
	for _, args := range [][]string{{}, {"--help"}, {"version"}, {"--version"}} {
		code, stdout, stderr := run(t, args...)
		want := exitOK
		if len(args) == 0 {
			want = exitUsage
		}
		if code != want || !strings.Contains(stderr, "DataTF v0.3.0 is available") {
			t.Fatalf("%v: %d %s", args, code, stderr)
		}
		if strings.Contains(stdout, "Upgrade:") {
			t.Fatal("notice changed stdout")
		}
		if !strings.Contains(stderr, "https://github.com/536tech/datatf/releases/tag/v0.3.0") {
			t.Fatal("missing release link")
		}
	}
	if *calls != 4 {
		t.Fatalf("checks: %d", *calls)
	}
}

func TestUpdateNoticeExclusions(t *testing.T) {
	calls := captureUpdateCheck(t)
	for _, args := range [][]string{
		{"version", "--json"}, {"version", "--plain"}, {"version", "--quiet"},
		{"telemetry", "status"}, {"telemetry", "preview"}, {"help", "telemetry"},
		{"completion", "bash"}, {"__complete", "export", ""},
		{"invalid-command"}, {"export", "--scope", "invalid"},
	} {
		_, _, stderr := run(t, args...)
		if strings.Contains(stderr, "Upgrade:") {
			t.Fatalf("notice for %v", args)
		}
	}
	if *calls != 0 {
		t.Fatalf("excluded commands made %d checks", *calls)
	}
}

func TestUpdateNoticeEnvironmentOptOut(t *testing.T) {
	calls := captureUpdateCheck(t)
	for _, key := range []string{
		"DATATF_NO_UPDATE_NOTIFIER", "DO_NOT_TRACK", "CI", "GITHUB_ACTIONS", "TF_BUILD",
		"GITLAB_CI", "JENKINS_URL", "CODEX_THREAD_ID", "CODEX_CI", "CLAUDECODE",
		"CLAUDE_CODE_ENTRYPOINT",
	} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "1")
			_, _, stderr := run(t, "version")
			if strings.Contains(stderr, "Upgrade:") {
				t.Fatal(stderr)
			}
		})
	}
	if *calls != 0 {
		t.Fatalf("opt-out made %d checks", *calls)
	}
}

func TestUpdateNoticeRequiresBothTerminals(t *testing.T) {
	calls := captureUpdateCheck(t)
	for _, failed := range []int{1, 2} {
		checks := 0
		terminalOutput = func(io.Writer) bool { checks++; return checks != failed }
		run(t, "version")
	}
	if *calls != 0 {
		t.Fatal("redirected output made a check")
	}
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if isTerminalOutput(file) || isTerminalOutput(io.Discard) {
		t.Fatal("not a terminal")
	}
}

func TestUpdateFailureDoesNotChangeExport(t *testing.T) {
	calls := captureUpdateCheck(t)
	isolateAuth(t, fakews.New(t))
	oldNow := now
	now = func() time.Time { return time.Unix(0, 0) }
	t.Cleanup(func() { now = oldNow })
	checkForUpdate = func(context.Context, string, string, time.Time) string {
		(*calls)++
		return ""
	}
	var baselineFiles [][]byte
	var baselineOutput string
	for _, disabled := range []string{"1", ""} {
		t.Setenv("DATATF_NO_UPDATE_NOTIFIER", disabled)
		dir := t.TempDir()
		code, stdout, stderr := run(t, "export", "--resources", "warehouses", "--out", dir)
		if code != exitOK {
			t.Fatalf("%d %s", code, stderr)
		}
		files := readExportFiles(t, dir)
		output := strings.ReplaceAll(stdout+stderr, dir, "<output-directory>")
		if disabled == "1" {
			baselineFiles, baselineOutput = files, output
		} else if !reflect.DeepEqual(files, baselineFiles) || output != baselineOutput {
			t.Fatal("failed update check changed export files or output")
		}
	}
	if *calls != 1 {
		t.Fatalf("checks: %d", *calls)
	}
}
