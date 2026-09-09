// Package emit writes export artifacts to disk.
package emit

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/536tech/datatf/internal/contract"
	"github.com/536tech/datatf/internal/inventory"
)

// WriteExport writes one export. It refuses to overwrite existing artifacts.
func WriteExport(dir string, ex *contract.Export, rep *contract.Report, rootModule string) (
	[]string, error,
) {
	files, err := ExportFiles(ex, rep, rootModule)
	if err != nil {
		return nil, err
	}
	return WriteFiles(dir, files)
}

// ExportFiles renders matching Terraform and JSON artifacts before any file write.
func ExportFiles(ex *contract.Export, rep *contract.Report, rootModule string) (
	map[string][]byte, error,
) {
	tfvars, err := contract.RenderVariables(ex.Tfvars.Variables(), contract.Header(ex.Scope, rep.Host))
	if err != nil {
		return nil, err
	}
	files, err := jsonFiles(map[string]any{"export.json": ex, "export-report.json": rep})
	if err != nil {
		return nil, err
	}
	files["terraform.tfvars"] = tfvars
	files["imports.tf"] = contract.RenderImports(ex.Imports, rootModule, contract.ImportsHeader(rep.Host))
	return files, nil
}

// WriteInventory writes inventory.json and inventory-report.json without overwriting files.
func WriteInventory(dir string, inv *inventory.Inventory, rep *contract.Report) ([]string, error) {
	files, err := jsonFiles(map[string]any{"inventory.json": inv, "inventory-report.json": rep})
	if err != nil {
		return nil, err
	}
	return WriteFiles(dir, files)
}

func jsonFiles(values map[string]any) (map[string][]byte, error) {
	files := map[string][]byte{}
	for name, value := range values {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", name, err)
		}
		files[name] = append(data, '\n')
	}
	return files, nil
}

// WriteFiles checks every destination before writing. Exclusive creation also
// protects files and symlinks created between the check and the write.
func WriteFiles(dir string, files map[string][]byte) ([]string, error) {
	names := slices.Sorted(maps.Keys(files))
	for _, name := range names {
		target := filepath.Join(dir, name)
		if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("refusing to overwrite or access %s; use a new directory", target)
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create output directory %s: %w", dir, err)
	}
	written := []string{}
	for _, name := range names {
		target := filepath.Join(dir, name)
		if err := writeNewFile(target, files[name]); err != nil {
			return written, err
		}
		written = append(written, target)
	}
	return written, nil
}

func writeNewFile(target string, data []byte) error {
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}
	return nil
}
