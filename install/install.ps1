# DataTF installer for Windows (PowerShell 5.1 or 7).
#   irm https://datatf.io/install.ps1 | iex
# Installs the latest release into %LOCALAPPDATA%\datatf\bin and adds it to the
# user PATH. Set $env:DATATF_VERSION, such as 1.0.0, to pin a release.
$ErrorActionPreference = "Stop"

$repo = "536tech/datatf"
$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
$version = $env:DATATF_VERSION
if ($version) {
    $base = "https://github.com/$repo/releases/download/v$($version.TrimStart('v'))"
} else {
    $base = "https://github.com/$repo/releases/latest/download"
}
$asset = "datatf_windows_${arch}.zip"
$dest = Join-Path $env:LOCALAPPDATA "datatf\bin"
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) "datatf-install-$([guid]::NewGuid())"

New-Item -ItemType Directory -Force -Path $dest, $tmp | Out-Null
try {
    Write-Host "Downloading $asset..."
    Invoke-WebRequest "$base/$asset" -OutFile (Join-Path $tmp $asset)
    Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $tmp "checksums.txt")

    $line = Select-String -Path (Join-Path $tmp "checksums.txt") -SimpleMatch " $asset" | Select-Object -First 1
    if (-not $line) { throw "checksums.txt has no entry for $asset" }
    $expected = $line.Line.Split(" ")[0].ToLower()
    $actual = (Get-FileHash (Join-Path $tmp $asset) -Algorithm SHA256).Hash.ToLower()
    if ($expected -ne $actual) { throw "checksum mismatch for $asset" }

    Expand-Archive (Join-Path $tmp $asset) -DestinationPath $tmp -Force
    Copy-Item (Join-Path $tmp "datatf.exe") (Join-Path $dest "datatf.exe") -Force
} finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ";") -notcontains $dest) {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$dest", "User")
    $env:Path = "$env:Path;$dest"
    Write-Host "Added $dest to your user PATH. Open a new terminal to use it."
}
& (Join-Path $dest "datatf.exe") version
