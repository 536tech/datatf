# Contributing

Open an issue for a bug or proposed resource type. Include a reproduction with credentials removed.
Use [the security policy](SECURITY.md) for security reports.

## Build and test

Install the Go version from `go.mod`:

```sh
git clone https://github.com/536tech/datatf.git
cd datatf
go build -trimpath -o bin/datatf ./cmd/datatf
go test ./...
./bin/datatf version
```

On Windows, use `bin/datatf.exe` as the build output. If you have Make, `make check` also runs
formatting, dependency checks, and vet.

Go tests use the fake API in `internal/fakews` to check reads, resource selection, and errors.
Telemetry tests use local HTTP servers and temporary consent files. They send no production events.
Contract changes also require `make e2e`. That check tests both scopes in both module layouts:

1. Terraform's [mock provider](https://developer.hashicorp.com/terraform/language/tests/mocking)
   checks the configuration with the real provider schema. This test excludes import blocks.
2. The real Databricks provider imports from the fake API into temporary local state.
   The test requires an imports-only plan and a clean second plan.

Keep provider validation and import logic in the provider. These tests do not prove cloud access
or permissions. Use real-workspace tests to check those behaviors.

`make e2e` needs Terraform, network access, and the module at `../terraform-databricks-workspace`.
To use the public module, run `scripts/e2e-fake.sh 536tech/workspace/databricks`, as CI does.
Neither command needs cloud credentials or an Azure subscription.

For a Docker test through the installed Databricks CLI and Terraform provider, run `make demo`.
Read [the MiniLake example](examples/minilake/README.md) for its scope and local cleanup command.

To update contract snapshots, run `go test ./internal/contract -run TestGolden -update`.
Review the generated diff. Open a pull request with the change and its test result.

## Releases

Add an entry under `## [Unreleased]` in [CHANGELOG.md](CHANGELOG.md) with each pull request.
Use [Semantic Versioning](https://semver.org/spec/v2.0.0.html) with tags named `vMAJOR.MINOR.PATCH`.
The tag is the version source. Use a patch for compatible fixes and a minor for compatible
features. A breaking CLI, module contract, or Terraform output change requires a major version.
Describe any breaking change in the pull request.

Merge the change and wait for CI on `main` to pass. From a clean checkout of `main`:

```sh
git pull --ff-only
git tag -a v1.0.3 -m "v1.0.3"
git push origin v1.0.3
```

Replace `v1.0.3` with the next version. Use `v1.1.0-rc.1` for a release candidate.
Never move or reuse a published tag. Use a new version for a correction.
The release title matches the tag, such as `v1.0.3`.

The tag starts GoReleaser through GitHub Actions. It tests the code and publishes six archives,
checksums, and release notes from merged pull requests. Release candidates receive the prerelease
label. The workflow uses the repository's `GITHUB_TOKEN`.

To test packaging locally, install [GoReleaser](https://goreleaser.com/install/).
Run `goreleaser check`, then `make release-snapshot`. The snapshot writes to `dist/` without a tag
or publication. A snapshot keeps a development version; it is not a release.
