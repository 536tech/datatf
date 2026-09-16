package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/536tech/datatf/internal/fakews"
)

func TestLatestModuleVersionRequiresScaffold(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	code, _, stderr := run(t, "export", "--module-version", "latest", "--out", filepath.Join(t.TempDir(), "out"))
	if code != exitUsage {
		t.Fatalf("latest without scaffold: exit %d: %s", code, stderr)
	}
}

func TestLatestModuleVersionsPinSelectedSources(t *testing.T) {
	for _, layout := range []string{"workspace", "resources"} {
		t.Run(layout, func(t *testing.T) {
			srv := fakews.New(t)
			isolateAuth(t, srv)
			old := resolveModuleVersions
			t.Cleanup(func() { resolveModuleVersions = old })
			var requested []string
			resolveModuleVersions = func(ctx context.Context, sources []string) (map[string]string, error) {
				requested = sources
				versions := map[string]string{}
				for i, source := range sources {
					versions[source] = fmt.Sprintf("1.2.%d", i)
				}
				return versions, nil
			}
			out := filepath.Join(t.TempDir(), "out")
			code, _, stderr := run(t, "export", "--resources", "catalogs", "--module-layout", layout, "--scaffold", "--module-version", "latest", "--out", out)
			if code != exitOK {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			file, diags := hclsyntax.ParseConfig(readGenerated(t, out, "main.tf"), "main.tf", hcl.InitialPos)
			if diags.HasErrors() {
				t.Fatal(diags)
			}
			blocks := file.Body.(*hclsyntax.Body).Blocks
			if len(requested) != len(blocks) || len(blocks) == 0 {
				t.Fatalf("lookups %v, blocks %d", requested, len(blocks))
			}
			for i, block := range blocks {
				value, _ := block.Body.Attributes["version"].Expr.Value(nil)
				if value.AsString() != fmt.Sprintf("1.2.%d", i) {
					t.Fatalf("version %v", value)
				}
				source, _ := block.Body.Attributes["source"].Expr.Value(nil)
				if source.AsString() != requested[i] {
					t.Fatalf("source %v != %s", source, requested[i])
				}
			}
		})
	}
}

func TestRegistryFailureWritesNothing(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	old := resolveModuleVersions
	t.Cleanup(func() { resolveModuleVersions = old })
	resolveModuleVersions = func(context.Context, []string) (map[string]string, error) {
		return nil, errors.New("Registry unavailable")
	}
	out := filepath.Join(t.TempDir(), "out")
	code, _, stderr := run(t, "export", "--json", "--scaffold", "--module-version", "latest", "--out", out)
	if code != exitErr || !strings.Contains(stderr, "module_version_error") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("Registry failure wrote output")
	}
}

func TestDefaultAndExplicitVersionDoNotResolve(t *testing.T) {
	old := resolveModuleVersions
	t.Cleanup(func() { resolveModuleVersions = old })
	resolveModuleVersions = func(context.Context, []string) (map[string]string, error) {
		t.Fatal("unexpected Registry call")
		return nil, nil
	}
	for _, version := range []string{"", "1.2.3"} {
		srv := fakews.New(t)
		isolateAuth(t, srv)
		args := []string{"export", "--scaffold", "--out", filepath.Join(t.TempDir(), "out")}
		if version != "" {
			args = append(args, "--module-version", version)
		}
		if code, _, stderr := run(t, args...); code != exitOK {
			t.Fatalf("exit %d: %s", code, stderr)
		}
	}
}

func TestLatestRejectsNonPublicSource(t *testing.T) {
	for _, source := range []string{"../module", "git::https://example.com/module.git", "private.example/team/module/databricks"} {
		code, _, stderr := run(t, "export", "--scaffold", "--module-source", source, "--module-version", "latest")
		if code != exitUsage {
			t.Fatalf("%s: exit %d: %s", source, code, stderr)
		}
	}
}
