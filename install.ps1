# install.ps1 — download and install the latest flow binary on Windows
# Usage:
#   irm https://raw.githubusercontent.com/benfo/flow/main/install.ps1 | iex
#   & ([scriptblock]::Create((irm .../install.ps1))) -PreRelease   # include pre-releases
#   $env:INSTALL_DIR = "C:\tools"; irm .../install.ps1 | iex       # override location
param(
  [switch]$PreRelease
)
$ErrorActionPreference = "Stop"

$Repo       = "benfo/flow"
$Binary     = "flow"
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { "$env:LOCALAPPDATA\flow" }

# ── Detect architecture ──────────────────────────────────────────────────────
$Arch = if ([System.Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }

# ── Resolve version ──────────────────────────────────────────────────────────
$Version = $env:VERSION
if (-not $Version) {
  if ($PreRelease) {
    # Pick the first entry from all releases (includes pre-releases).
    $releases = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases"
    $Version  = $releases[0].tag_name -replace '^v', ''
  } else {
    # Try stable release first.
    try {
      $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
      $Version = $release.tag_name -replace '^v', ''
    } catch {}

    # No stable release yet — fall back to the latest pre-release automatically.
    if (-not $Version) {
      Write-Host "  No stable release found, falling back to latest pre-release..."
      $releases = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases"
      $Version  = $releases[0].tag_name -replace '^v', ''
    }
  }
}

if (-not $Version) {
  Write-Error "Could not determine a release version. Set `$env:VERSION = 'x.y.z' to override."
  exit 1
}

# ── Download ─────────────────────────────────────────────────────────────────
$ZipName = "${Binary}_${Version}_windows_${Arch}.zip"
$Url     = "https://github.com/$Repo/releases/download/v$Version/$ZipName"

Write-Host "  Downloading flow v$Version (windows/$Arch)..."

$Tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $Tmp | Out-Null

try {
  $ZipPath = Join-Path $Tmp $ZipName
  Invoke-WebRequest -Uri $Url -OutFile $ZipPath -UseBasicParsing
  Expand-Archive -Path $ZipPath -DestinationPath $Tmp

  # ── Install ─────────────────────────────────────────────────────────────
  if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
  }
  Copy-Item (Join-Path $Tmp "$Binary.exe") (Join-Path $InstallDir "$Binary.exe") -Force

  # ── Add to user PATH if missing ──────────────────────────────────────────
  $UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
  if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
    Write-Host "  Added $InstallDir to your PATH."
    Write-Host "  Restart your terminal for the PATH change to take effect."
  }

  Write-Host "  ✓ Installed flow v$Version → $InstallDir\$Binary.exe"
}
finally {
  Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
}
