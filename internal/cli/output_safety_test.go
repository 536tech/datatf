package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/536tech/datatf/internal/fakews"
)

func TestExportPreservesExistingRoot(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	for _, name := range []string{"terraform.tfvars", "main.tf"} {
		t.Run(name, func(t *testing.T) {
			out := t.TempDir()
			existing := filepath.Join(out, name)
			if err := os.WriteFile(existing, []byte("keep this\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			code, _, stderr := run(t, "export", "--out", out, "--scaffold")
			if code != exitErr || !strings.Contains(stderr, "refusing to overwrite") {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			entries, err := os.ReadDir(out)
			if err != nil || len(entries) != 1 {
				t.Fatalf("failed export changed root: %v %v", entries, err)
			}
			data, err := os.ReadFile(existing)
			if err != nil || string(data) != "keep this\n" {
				t.Fatalf("existing file changed: %q %v", data, err)
			}
		})
	}
}
