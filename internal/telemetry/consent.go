package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Status describes effective consent without creating files or contacting a server.
type Status struct {
	Enabled     bool   `json:"enabled"`
	Source      string `json:"source"`
	ConfigFile  string `json:"config_file"`
	ConfigError string `json:"config_error,omitempty"`
}

type consent struct {
	SchemaVersion int  `json:"schema_version"`
	Enabled       bool `json:"enabled"`
}

// Resolve gives environment opt-outs priority over saved consent.
// An explicit environment opt-in is required in detected automation sessions.
// Without a saved preference, telemetry is on. Invalid or unreadable settings turn it off.
func Resolve(path string, getenv func(string) string) Status {
	status := Status{Source: "default", ConfigFile: path}
	if value := getenv("DO_NOT_TRACK"); value == "1" || strings.EqualFold(value, "true") {
		status.Source = "DO_NOT_TRACK"
		return status
	}
	if value := getenv("DATATF_TELEMETRY"); value != "" {
		status.Enabled, status.Source = value == "1", "DATATF_TELEMETRY"
		return status
	}
	if automation(getenv) {
		status.Source = "automation"
		return status
	}
	enabled, err := readConsent(path)
	if os.IsNotExist(err) {
		status.Enabled = true
		return status
	}
	status.Source = "config"
	if err != nil {
		status.ConfigError = err.Error()
		return status
	}
	status.Enabled = enabled
	return status
}

func automation(getenv func(string) string) bool {
	for _, key := range []string{
		"CI", "GITHUB_ACTIONS", "TF_BUILD", "GITLAB_CI", "JENKINS_URL",
		"CODEX_THREAD_ID", "CODEX_CI", "CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT",
	} {
		if value := getenv(key); value != "" && value != "0" && value != "false" {
			return true
		}
	}
	return false
}

func readConsent(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return false, fmt.Errorf("telemetry settings must be a regular file of at most 4096 bytes")
	}
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 4097))
	decoder.DisallowUnknownFields()
	var saved consent
	if err := decoder.Decode(&saved); err != nil {
		return false, fmt.Errorf("invalid telemetry settings")
	}
	if decoder.Decode(new(any)) != io.EOF || saved.SchemaVersion != 1 {
		return false, fmt.Errorf("invalid telemetry settings schema")
	}
	return saved.Enabled, nil
}

// SaveConsent replaces only DataTF's consent file. Invalid settings turn telemetry off.
func SaveConsent(path string, enabled bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".telemetry-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	writeErr := json.NewEncoder(file).Encode(consent{SchemaVersion: 1, Enabled: enabled})
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}
