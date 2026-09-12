// Package telemetry builds optional usage events without workspace metadata.
package telemetry

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Run contains only the inputs permitted in a usage event.
type Run struct {
	Version, OS, Arch, Command, Scope, Outcome, ErrorCode string
	ResourceGroups                                        []string
	Duration                                              time.Duration
}

type event struct {
	SchemaVersion  int      `json:"schema_version"`
	Event          string   `json:"event"`
	Version        string   `json:"version"`
	OS             string   `json:"os"`
	Arch           string   `json:"arch"`
	Command        string   `json:"command"`
	Scope          string   `json:"scope"`
	Outcome        string   `json:"outcome"`
	ErrorCode      string   `json:"error_code"`
	DurationBucket string   `json:"duration_bucket"`
	ResourceGroups []string `json:"resource_groups"`
}

var groups = []string{
	"catalogs", "storage_credentials", "external_locations", "cluster_policies",
	"instance_pools", "warehouses", "secret_scopes", "service_principals",
}

var codes = []string{
	"none", "other", "invalid_usage", "configuration_error", "authentication_failed",
	"permission_denied", "not_found", "resource_exhausted", "service_error", "api_error",
	"connection_failed", "operation_failed", "timeout", "canceled", "partial_result", "output_error",
	"workspace_error", "render_error",
}

var releaseVersion = regexp.MustCompile(
	`^\d{1,4}\.\d{1,4}\.\d{1,4}(?:-(?:alpha|beta|rc)\.\d{1,4})?$`,
)

// Payload returns the same allowlisted JSON for delivery and offline preview.
// Unknown metadata is replaced, never copied from a report or command line.
func Payload(run Run) ([]byte, error) {
	if !slices.Contains([]string{"inventory", "export"}, run.Command) {
		return nil, fmt.Errorf("telemetry accepts only inventory or export")
	}
	scope := allowed(run.Scope, "none", "workspace", "shared", "none")
	if run.Command == "inventory" {
		scope = "none"
	}
	selected := make([]string, 0, len(groups))
	for _, group := range groups {
		if slices.Contains(run.ResourceGroups, group) {
			selected = append(selected, group)
		}
	}
	slices.Sort(selected)
	return json.Marshal(event{
		SchemaVersion: 1, Event: "command_completed", Version: safeVersion(run.Version),
		OS:      allowed(run.OS, "other", "darwin", "windows", "linux"),
		Arch:    allowed(run.Arch, "other", "amd64", "arm64", "386", "arm"),
		Command: run.Command, Scope: scope,
		Outcome:        allowed(run.Outcome, "error", "complete", "partial", "error", "canceled"),
		ErrorCode:      allowed(run.ErrorCode, "other", codes...),
		DurationBucket: durationBucket(run.Duration), ResourceGroups: selected,
	})
}

func safeVersion(version string) string {
	version = strings.TrimPrefix(version, "v")
	if !releaseVersion.MatchString(version) {
		return "dev"
	}
	return version
}

func allowed(value, fallback string, values ...string) string {
	if slices.Contains(values, value) {
		return value
	}
	return fallback
}

func durationBucket(duration time.Duration) string {
	switch {
	case duration < time.Second:
		return "under_1s"
	case duration < 10*time.Second:
		return "1s_to_10s"
	case duration <= time.Minute:
		return "10s_to_60s"
	default:
		return "over_60s"
	}
}
