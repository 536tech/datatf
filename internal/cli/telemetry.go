package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/536tech/datatf/internal/telemetry"
)

var userConfigDir = os.UserConfigDir
var sendUsage = telemetry.Send

func newTelemetryCommand(rc *runtime) *cobra.Command {
	root := &cobra.Command{
		Use: "telemetry", Short: "Control optional usage metrics (on by default)",
		Args: usageArgs(cobra.NoArgs),
		Long: "Telemetry is on by default outside CI and agent sessions. Read " + telemetry.Notice + ".\n" +
			"Run `datatf telemetry disable` or set DATATF_TELEMETRY=0 to opt out.\n" +
			"These commands work offline and do not read a Databricks workspace.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commandRequired(cmd, rc)
		},
	}
	for _, action := range []string{"enable", "disable", "status", "preview"} {
		root.AddCommand(&cobra.Command{
			Use: action, Args: usageArgs(cobra.NoArgs),
			Short: map[string]string{
				"enable":  "Save consent to optional command metrics",
				"disable": "Save a disabled telemetry preference",
				"status":  "Show effective consent and its source",
				"preview": "Print a sample event offline with this binary's version and platform",
			}[action],
			RunE: func(cmd *cobra.Command, args []string) error { return rc.telemetryAction(action) },
		})
	}
	return root
}

func telemetryStatus() telemetry.Status {
	dir, err := userConfigDir()
	path := ""
	if err == nil {
		path = filepath.Join(dir, "datatf", "telemetry.json")
	}
	status := telemetry.Resolve(path, os.Getenv)
	if err != nil {
		status.ConfigError = err.Error()
	}
	return status
}

func (rc *runtime) telemetryAction(action string) error {
	if action == "preview" {
		return rc.previewUsage()
	}
	status := telemetryStatus()
	if action != "status" {
		if status.ConfigFile == "" {
			return withHint(fmt.Errorf("cannot locate the user configuration directory"),
				"configuration_error", "Set DATATF_TELEMETRY=0 to disable telemetry for this process.")
		}
		if err := telemetry.SaveConsent(status.ConfigFile, action == "enable"); err != nil {
			return withHint(fmt.Errorf("save telemetry preference: %w", err), "configuration_error",
				"Check access to the user configuration directory. Use DATATF_TELEMETRY=0 to opt out.")
		}
		status = telemetryStatus()
	}
	if rc.out.IsJSON() {
		return rc.out.JSON(status)
	}
	if action != "status" {
		rc.out.Printf("saved telemetry preference: %s\n", action)
	}
	rc.out.Printf("telemetry enabled: %t (%s)\n", status.Enabled, status.Source)
	rc.out.Printf("settings: %s\nendpoint: %s\n", status.ConfigFile, telemetry.Endpoint)
	rc.out.Printf("retention: three months\nnotice: %s\n", telemetry.Notice)
	if status.ConfigError != "" {
		rc.out.Printf("settings error: %s\n", terminalText(status.ConfigError))
	}
	return nil
}

// writeTelemetryNotice tells interactive users about default telemetry until they save a preference.
func (rc *runtime) writeTelemetryNotice() {
	_, _ = fmt.Fprintf(rc.stderr,
		"\nDataTF sends optional usage metrics without workspace metadata. Notice: %s\n"+
			"Run `datatf telemetry disable` or set DATATF_TELEMETRY=0 to opt out.\n",
		telemetry.Notice)
}

func (rc *runtime) previewUsage() error {
	payload, err := telemetry.Payload(telemetry.Run{
		Version: version, OS: goruntime.GOOS, Arch: goruntime.GOARCH,
		Command: "export", Scope: "workspace", ResourceGroups: []string{"catalogs"},
		Outcome: "complete", ErrorCode: "none", Duration: 2 * time.Second,
	})
	if err != nil {
		return err
	}
	return rc.out.JSON(json.RawMessage(payload))
}

func (rc *runtime) beginUsage(command string) {
	if command != "inventory" && command != "export" {
		return
	}
	rc.usage = &telemetry.Run{
		Version: version, OS: goruntime.GOOS, Arch: goruntime.GOARCH,
		Command: command, Scope: "none", Outcome: "complete", ErrorCode: "none",
	}
	rc.usageStarted = time.Now()
}

func (rc *runtime) finishUsage(failed bool) {
	if rc.usage == nil {
		return
	}
	status := telemetryStatus()
	if !status.Enabled {
		return
	}
	if status.Source == "default" && rc.updateOutputAllowed() {
		rc.writeTelemetryNotice()
	}
	if failed {
		rc.usage.Outcome = "error"
		switch rc.usage.ErrorCode {
		case "partial_result":
			rc.usage.Outcome = "partial"
		case "canceled":
			rc.usage.Outcome = "canceled"
		}
	}
	rc.usage.Duration = time.Since(rc.usageStarted)
	sendUsage(rc.ctx, *rc.usage)
}
