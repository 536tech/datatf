# Install DataTF

DataTF is a single binary. It needs no Go toolchain and no administrator access.

## Installer script

macOS and Linux:

```sh
curl -fsSL https://datatf.io/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://datatf.io/install.ps1 | iex
```

The scripts download the latest GitHub release, verify its SHA-256 checksum, and install one
binary. Set `DATATF_VERSION` to pin a release, such as `1.0.2`. The sources live in
[install](../install).

macOS and Linux install into `~/.local/bin`. Set `DATATF_INSTALL_DIR` to select a different
directory. When the script reports that the directory is not on your `PATH`, add it:

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
exec zsh
```

Windows installs into `%LOCALAPPDATA%\datatf\bin` and adds that directory to your user `PATH`.
Open a new terminal after the first install.

## macOS Gatekeeper

DataTF binaries are not signed with an Apple Developer ID. macOS marks files downloaded through a
browser, so an archive from the Releases page or the website carries the quarantine flag. The first
run then reports that Apple cannot verify the developer.

The installer script above avoids this, because `curl` does not set the quarantine flag.

To use a browser download, remove the flag after you extract the archive:

```sh
xattr -d com.apple.quarantine ./datatf
```

You can also open the containing folder in Finder, hold Control, click `datatf`, and select **Open**.
Confirm the dialog once. macOS then permits later runs.

## Download a release

1. Open [GitHub Releases](https://github.com/536tech/datatf/releases/latest).
2. Download the archive for your operating system and processor.
   Releases include macOS, Linux, and Windows builds for AMD64 and ARM64.
3. Compare the archive's SHA-256 checksum with the value in `checksums.txt` from the same release.
4. Extract the archive.
5. On macOS, remove the quarantine flag. See [macOS Gatekeeper](#macos-gatekeeper).
6. Put `datatf` on your `PATH`.
7. Run `datatf version` to check the installation.

## Windows

Download the Windows AMD64 ZIP for an Intel or AMD computer. Use the ARM64 ZIP for a Windows ARM
computer. Select **Extract All**, then open PowerShell in the extracted folder:

```powershell
.\datatf.exe version
.\datatf.exe auth status --profile analytics
.\datatf.exe export --profile analytics --scaffold
```

Replace `analytics` with your saved Databricks profile.
Add the folder to your user `PATH` to run `datatf` from any folder.

## Terraform

DataTF writes Terraform files. It does not run Terraform.
Install [Terraform](https://developer.hashicorp.com/terraform/install) only when you are ready to
validate and import the generated files. The generated root declares its own required version.
Run `terraform version` to compare your installation with that requirement.
OpenTofu also works. See the [OpenTofu example](../examples/opentofu/README.md).

## Build from source

Read [Contributing](../CONTRIBUTING.md) for the source build, the tests, and the release tags.

## Updates

Interactive commands check GitHub Releases for a newer stable release.
Read the [upgrade instructions](updates.md) for the steps, the check behavior, and
`DATATF_NO_UPDATE_NOTIFIER`.
