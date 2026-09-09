package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/536tech/datatf/internal/update"
)

var checkForUpdate = update.Check
var updateCacheDir = os.UserCacheDir
var terminalOutput = isTerminalOutput

func (rc *runtime) updateNotice(cmd *cobra.Command) {
	if !rc.updateOutputAllowed() || updateDisabled() {
		return
	}
	for current := cmd; current != nil; current = current.Parent() {
		switch current.Name() {
		case "telemetry", "completion", "__complete", "__completeNoDesc", "help":
			return
		}
	}
	rc.writeUpdateNotice()
}

func (rc *runtime) updateOutputAllowed() bool {
	return !rc.g.asJSON && !rc.g.plain && !rc.g.quiet &&
		terminalOutput(rc.stdout) && terminalOutput(rc.stderr)
}

func updateDisabled() bool {
	if os.Getenv("DATATF_NO_UPDATE_NOTIFIER") != "" {
		return true
	}
	if value := os.Getenv("DO_NOT_TRACK"); value == "1" || strings.EqualFold(value, "true") {
		return true
	}
	for _, key := range []string{
		"CI", "GITHUB_ACTIONS", "TF_BUILD", "GITLAB_CI", "JENKINS_URL",
		"CODEX_THREAD_ID", "CODEX_CI", "CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT",
	} {
		if value := os.Getenv(key); value != "" && value != "0" && value != "false" {
			return true
		}
	}
	return false
}

func (rc *runtime) writeUpdateNotice() {
	dir, err := updateCacheDir()
	if err != nil {
		return
	}
	latest := checkForUpdate(rc.ctx, version, filepath.Join(dir, "datatf", "update.json"), now())
	if latest == "" {
		return
	}
	binary := "datatf"
	if goruntime.GOOS == "windows" {
		binary = "datatf.exe"
	}
	_, _ = fmt.Fprintf(rc.stderr,
		"\nDataTF %s is available (installed: %s).\nUpgrade: %s%s\n"+
			"Download the archive for your system. Verify its checksum. Replace %s on PATH.\n",
		latest, version, update.Releases, latest, binary)
}

func isTerminalOutput(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}
