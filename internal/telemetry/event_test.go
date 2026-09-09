package telemetry

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func sampleRun() Run {
	return Run{
		Version: "0.3.0", OS: "darwin", Arch: "arm64", Command: "export", Scope: "workspace",
		Outcome: "complete", ErrorCode: "none", Duration: 2 * time.Second,
		ResourceGroups: []string{"warehouses", "catalogs"},
	}
}

func TestPayloadContract(t *testing.T) {
	payload, err := Payload(sampleRun())
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/event.json")
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected any
	if err := json.Unmarshal(payload, &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) || len(payload) > 2048 {
		t.Fatalf("payload does not match v1 contract: %s", payload)
	}
}

func TestPayloadRejectsOrReplacesUntrustedValues(t *testing.T) {
	run := Run{
		Version: "0.3.0+canary", OS: "canary", Arch: "canary", Command: "export",
		Scope: "canary", Outcome: "canary", ErrorCode: "permission_denied: canary",
		ResourceGroups: []string{"canary", "catalogs", "catalogs"},
	}
	payload, err := Payload(run)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "canary") {
		t.Fatalf("unsafe value entered event: %s", payload)
	}
	var event event
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}
	if event.Version != "dev" || event.ErrorCode != "other" || len(event.ResourceGroups) != 1 {
		t.Fatalf("invalid normalization: %+v", event)
	}
}

func TestPayloadRejectsUnsupportedCommand(t *testing.T) {
	run := sampleRun()
	run.Command = "canary"
	if payload, err := Payload(run); err == nil || payload != nil {
		t.Fatal("unsupported commands must not produce events")
	}
}

func TestPayloadDefaultsAndInventoryScope(t *testing.T) {
	run := sampleRun()
	run.Command, run.ResourceGroups = "inventory", nil
	payload, err := Payload(run)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"scope":"none"`) ||
		!strings.Contains(string(payload), `"resource_groups":[]`) {
		t.Fatalf("invalid inventory event: %s", payload)
	}
}

func TestVersionsAndDurationBoundaries(t *testing.T) {
	for version, want := range map[string]string{
		"v0.3.0": "0.3.0", "1.2.3-rc.1": "1.2.3-rc.1", "dev": "dev",
		"0.3.1-next": "dev", "1.2.3+private": "dev", "10000.1.1": "dev",
	} {
		if got := safeVersion(version); got != want {
			t.Errorf("version %q: got %q, want %q", version, got, want)
		}
	}
	for duration, want := range map[time.Duration]string{
		0: "under_1s", time.Second - 1: "under_1s", time.Second: "1s_to_10s",
		10*time.Second - 1: "1s_to_10s", 10 * time.Second: "10s_to_60s",
		time.Minute: "10s_to_60s", time.Minute + 1: "over_60s",
	} {
		if got := durationBucket(duration); got != want {
			t.Errorf("duration %v: got %q, want %q", duration, got, want)
		}
	}
}
