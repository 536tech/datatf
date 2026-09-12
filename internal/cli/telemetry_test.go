package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/536tech/datatf/internal/fakews"
	"github.com/536tech/datatf/internal/telemetry"
)

func TestMain(m *testing.M) {
	// Tests must never inherit permission to contact the production collector.
	if err := os.Setenv("DATATF_TELEMETRY", "0"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func captureUsage(t *testing.T) *[]telemetry.Run {
	t.Helper()
	dir := t.TempDir()
	oldConfigDir, oldSender := userConfigDir, sendUsage
	userConfigDir = func() (string, error) { return dir, nil }
	var events []telemetry.Run
	sendUsage = func(_ context.Context, event telemetry.Run) { events = append(events, event) }
	t.Cleanup(func() { userConfigDir, sendUsage = oldConfigDir, oldSender })
	for _, key := range []string{
		"DO_NOT_TRACK", "CI", "GITHUB_ACTIONS", "TF_BUILD", "GITLAB_CI", "JENKINS_URL",
		"CODEX_THREAD_ID", "CODEX_CI", "CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("DATATF_TELEMETRY", "0")
	return &events
}

func TestTelemetryControlsAreOffline(t *testing.T) {
	events := captureUsage(t)
	t.Setenv("DATATF_TELEMETRY", "")
	for _, test := range []struct {
		action  string
		enabled bool
	}{{"status", true}, {"enable", true}, {"status", true}, {"disable", false}, {"status", false}} {
		status := runTelemetryStatus(t, test.action)
		if status.Enabled != test.enabled {
			t.Fatalf("%s: unexpected consent: %+v", test.action, status)
		}
	}
	code, preview, stderr := run(t, "telemetry", "preview")
	if code != exitOK || stderr != "" || !json.Valid([]byte(preview)) {
		t.Fatalf("preview: %d %s %s", code, preview, stderr)
	}
	if len(*events) != 0 {
		t.Fatal("consent controls sent an event")
	}
}

func runTelemetryStatus(t *testing.T, action string) telemetry.Status {
	t.Helper()
	code, stdout, stderr := run(t, "telemetry", action, "--json")
	if code != exitOK || stderr != "" {
		t.Fatalf("%s: exit %d: %s", action, code, stderr)
	}
	var status telemetry.Status
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatal(err)
	}
	return status
}

func TestTelemetryStatusAndPreviewDoNotCreateFiles(t *testing.T) {
	captureUsage(t)
	for _, action := range []string{"status", "preview"} {
		code, _, stderr := run(t, "telemetry", action)
		if code != exitOK {
			t.Fatalf("%s: %s", action, stderr)
		}
	}
	if _, err := os.Stat(telemetryStatus().ConfigFile); !os.IsNotExist(err) {
		t.Fatalf("read-only controls created consent: %v", err)
	}
}

func TestTelemetryUsageErrors(t *testing.T) {
	events := captureUsage(t)
	for _, args := range [][]string{
		{"telemetry"}, {"telemetry", "unknown"}, {"telemetry", "enable", "extra"},
	} {
		code, stdout, stderr := run(t, append([]string{"--json"}, args...)...)
		if code != exitUsage || stdout != "" {
			t.Fatalf("%v: exit %d: %s %s", args, code, stdout, stderr)
		}
		if failure := decodeFailure(t, stderr); failure["code"] != "invalid_usage" {
			t.Fatalf("unexpected diagnostic: %v", failure)
		}
	}
	if len(*events) != 0 {
		t.Fatal("telemetry usage error sent an event")
	}
}

func TestTelemetryOptOutAndCommandExclusions(t *testing.T) {
	events := captureUsage(t)
	isolateAuth(t, fakews.New(t))
	for _, mode := range []string{"", "0", "invalid", "1"} {
		t.Setenv("DATATF_TELEMETRY", mode)
		t.Setenv("DO_NOT_TRACK", "1")
		code, _, stderr := run(t, "inventory", "--resources", "warehouses", "--json")
		if code != exitOK {
			t.Fatalf("inventory: %s", stderr)
		}
	}
	t.Setenv("DO_NOT_TRACK", "")
	for _, args := range [][]string{
		{"version"}, {"--help"}, {"export", "--help"}, {"auth", "status"},
		{"completion", "powershell"}, {"telemetry", "status"}, {"telemetry", "preview"},
	} {
		code, _, stderr := run(t, args...)
		if code != exitOK {
			t.Fatalf("%v: %s", args, stderr)
		}
	}
	if len(*events) != 0 {
		t.Fatalf("unexpected events: %v", *events)
	}
}

func TestTelemetryRequiresExplicitConsentInAutomation(t *testing.T) {
	events := captureUsage(t)
	isolateAuth(t, fakews.New(t))
	t.Setenv("DATATF_TELEMETRY", "")
	t.Setenv("CODEX_THREAD_ID", "private-thread-canary")
	code, _, _ := run(t, "telemetry", "enable")
	if code != exitOK || telemetryStatus().Enabled {
		t.Fatal("saved consent must not enable a detected agent session")
	}
	run(t, "inventory", "--resources", "warehouses", "--json")
	if len(*events) != 0 {
		t.Fatal("agent inherited saved consent")
	}
	t.Setenv("DATATF_TELEMETRY", "1")
	run(t, "inventory", "--resources", "warehouses", "--json")
	if len(*events) != 1 {
		t.Fatal("explicit environment consent did not enable delivery")
	}
}

func TestTelemetryUsesSelectedGroupsAndScope(t *testing.T) {
	events := captureUsage(t)
	isolateAuth(t, fakews.New(t))
	t.Setenv("DATATF_TELEMETRY", "1")
	for _, scope := range []string{"workspace", "shared"} {
		code, _, stderr := run(t, "export", "--json", "--scope", scope,
			"--resources", "catalogs", "--out", t.TempDir())
		if code != exitOK {
			t.Fatalf("%s: %s", scope, stderr)
		}
		got := (*events)[len(*events)-1]
		if got.Scope != scope || !reflect.DeepEqual(got.ResourceGroups, []string{"catalogs"}) {
			t.Fatalf("incorrect selection: %+v", got)
		}
	}
	if len(*events) != 2 {
		t.Fatalf("expected two events: %v", *events)
	}
	assertUsageOutcome(t, *events, "complete", "none")
}

func TestTelemetryInventoryExcludesObjectMetadata(t *testing.T) {
	events := captureUsage(t)
	isolateAuth(t, fakews.New(t))
	t.Setenv("DATATF_TELEMETRY", "1")
	code, _, stderr := run(t, "inventory", "--json", "--resources", "catalogs", "--name", "sales")
	if code != exitOK {
		t.Fatal(stderr)
	}
	if len(*events) != 1 || (*events)[0].Scope != "none" || (*events)[0].Command != "inventory" {
		t.Fatalf("unexpected inventory event: %v", *events)
	}
	assertUsageOutcome(t, *events, "complete", "none")
	payload, _ := telemetry.Payload((*events)[0])
	for _, private := range []string{"sales", "jon@example.com", "127.0.0.1", "profile", "token"} {
		if strings.Contains(string(payload), private) {
			t.Fatalf("payload contains private data: %s", payload)
		}
	}
}

func TestTelemetryPartialResultsWithAndWithoutAllowPartial(t *testing.T) {
	events := captureUsage(t)
	srv := fakews.New(t)
	isolateAuth(t, srv)
	srv.Fail("GET", "/api/2.0/sql/warehouses", http.StatusForbidden, "private-error-canary")
	t.Setenv("DATATF_TELEMETRY", "1")
	for _, allow := range []bool{false, true} {
		args := []string{"export", "--json", "--resources", "warehouses", "--out", t.TempDir()}
		want := exitErr
		if allow {
			args, want = append(args, "--allow-partial"), exitOK
		}
		if code, _, stderr := run(t, args...); code != want {
			t.Fatalf("allow partial %t: %d %s", allow, code, stderr)
		}
	}
	if code, _, _ := run(t, "inventory", "--json", "--resources", "warehouses"); code != exitErr {
		t.Fatal("partial inventory must fail")
	}
	if len(*events) != 3 {
		t.Fatalf("expected three events: %v", *events)
	}
	assertUsageOutcome(t, *events, "partial", "partial_result")
}

func assertUsageOutcome(t *testing.T, events []telemetry.Run, outcome, code string) {
	t.Helper()
	for _, event := range events {
		if event.Outcome != outcome || event.ErrorCode != code {
			t.Fatalf("unexpected outcome: %+v", event)
		}
	}
}

func TestTelemetryFatalErrorsAndCancellation(t *testing.T) {
	events := captureUsage(t)
	srv := fakews.New(t)
	isolateAuth(t, srv)
	t.Setenv("DATATF_TELEMETRY", "1")
	srv.Fail("GET", "/api/2.0/preview/scim/v2/Me", http.StatusUnauthorized, "private-error-canary")
	run(t, "inventory", "--json", "--resources", "catalogs")
	assertUsageOutcome(t, *events, "error", "authentication_failed")
	run(t, "export", "--scope", "private-scope-canary")
	assertUsageOutcome(t, (*events)[1:], "error", "invalid_usage")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	Execute(ctx, []string{"inventory", "--json"}, strings.NewReader(""), io.Discard, io.Discard)
	if len(*events) != 3 {
		t.Fatalf("expected three failures: %v", *events)
	}
	assertUsageOutcome(t, (*events)[2:], "canceled", "canceled")
	for _, event := range *events {
		payload, _ := telemetry.Payload(event)
		if strings.Contains(string(payload), "canary") {
			t.Fatalf("raw error or argument entered payload: %s", payload)
		}
	}
}

func TestTelemetryDoesNotChangeExportArtifactsOrOutput(t *testing.T) {
	events := captureUsage(t)
	isolateAuth(t, fakews.New(t))
	oldNow := now
	now = func() time.Time { return time.Unix(0, 0) }
	t.Cleanup(func() { now = oldNow })
	var firstOutput string
	var firstFiles [][]byte
	for _, enabled := range []string{"0", "1"} {
		t.Setenv("DATATF_TELEMETRY", enabled)
		dir := t.TempDir()
		code, stdout, stderr := run(t, "export", "--json", "--resources", "warehouses", "--out", dir)
		if code != exitOK || stderr != "" {
			t.Fatalf("export: %d %s", code, stderr)
		}
		files := readExportFiles(t, dir)
		if enabled == "0" {
			firstOutput, firstFiles = stdout, files
		} else if firstOutput != stdout || !reflect.DeepEqual(firstFiles, files) {
			t.Fatal("telemetry changed stdout or export files")
		}
	}
	if len(*events) != 1 {
		t.Fatal("expected only the enabled export to send")
	}
}

func readExportFiles(t *testing.T, dir string) [][]byte {
	t.Helper()
	var files [][]byte
	for _, name := range []string{
		"terraform.tfvars", "imports.tf", "export.json", "export-report.json",
	} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, data)
	}
	return files
}

func TestTelemetryOutputErrorIsCoarse(t *testing.T) {
	events := captureUsage(t)
	isolateAuth(t, fakews.New(t))
	t.Setenv("DATATF_TELEMETRY", "1")
	var stderr bytes.Buffer
	code := Execute(context.Background(), []string{"inventory", "--json", "--resources", "warehouses"},
		strings.NewReader(""), brokenOutput{}, &stderr)
	if code != exitErr || len(*events) != 1 {
		t.Fatalf("output failure: %d %s", code, &stderr)
	}
	assertUsageOutcome(t, *events, "error", "operation_failed")
}

func TestTelemetryNoticeUntilPreferenceSaved(t *testing.T) {
	events := captureUsage(t)
	isolateAuth(t, fakews.New(t))
	t.Setenv("DATATF_TELEMETRY", "")
	t.Setenv("DATATF_NO_UPDATE_NOTIFIER", "1")
	oldTerminal := terminalOutput
	terminalOutput = func(io.Writer) bool { return false }
	t.Cleanup(func() { terminalOutput = oldTerminal })
	out := func() string { return t.TempDir() }

	// The notice must reach redirected, --json, --plain, and --quiet sessions before any send.
	for _, flag := range []string{"", "--json", "--plain", "--quiet"} {
		args := []string{"inventory", "--resources", "warehouses", "--out", out()}
		if flag != "" {
			args = append(args, flag)
		}
		code, stdout, stderr := run(t, args...)
		if code != exitOK || !strings.Contains(stderr, "datatf telemetry disable") ||
			!strings.Contains(stderr, telemetry.Notice) {
			t.Fatalf("%v: default consent must print a notice: %d %s", args, code, stderr)
		}
		if strings.Contains(stdout, "telemetry") {
			t.Fatalf("%v: notice changed stdout", args)
		}
	}
	if len(*events) != 4 {
		t.Fatalf("default consent must send one event per run: %v", *events)
	}
	if _, _, stderr := run(t, "telemetry", "status"); strings.Contains(stderr, "telemetry disable") {
		t.Fatalf("notice for an offline control: %s", stderr)
	}

	// A command that fails after it starts still prints the notice before its event is sent.
	code, _, stderr := run(t, "export", "--scope", "invalid", "--out", out())
	if code == exitOK || !strings.Contains(stderr, "telemetry disable") || len(*events) != 5 {
		t.Fatalf("failed command must notify before sending: %d %s %d", code, stderr, len(*events))
	}

	for _, action := range []string{"enable", "disable"} {
		run(t, "telemetry", action)
		if _, _, stderr := run(t, "inventory", "--resources", "warehouses", "--out", out()); strings.Contains(stderr, "telemetry disable") {
			t.Fatalf("notice after saved %s: %s", action, stderr)
		}
	}
	if len(*events) != 6 {
		t.Fatalf("expected events for default and enabled runs only: %d", len(*events))
	}
}
