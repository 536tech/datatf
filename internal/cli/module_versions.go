package cli

import (
	"context"

	"github.com/536tech/datatf/internal/contract"
	"github.com/536tech/datatf/internal/scaffold"
)

var resolveModuleVersions = scaffold.LatestVersions

func (opts *exportOptions) resolveVersions(ctx context.Context, ex *contract.Export) error {
	if opts.moduleVer != "latest" {
		return nil
	}
	sources := []string{opts.moduleSource}
	if opts.moduleLayout == "resources" {
		modules, err := contract.ResourceModules(ex.Tfvars)
		if err != nil {
			return err
		}
		sources = nil
		for _, module := range modules {
			sources = append(sources, module.Source)
		}
	}
	versions, err := resolveModuleVersions(ctx, sources)
	if err != nil {
		return err
	}
	if opts.moduleLayout == "resources" {
		opts.moduleVersions = versions
	} else {
		opts.moduleVer = versions[opts.moduleSource]
	}
	return nil
}
