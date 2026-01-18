# Usage:
#   powershell -ExecutionPolicy ByPass -File install.ps1
#   powershell -ExecutionPolicy ByPass -File install.ps1 -Version v1.0.0

param (
    [string]$Version
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

function Show-Error   { param([string]$Msg); Write-Host "error: $Msg" -ForegroundColor Red }
function Show-Success { param([string]$Msg); Write-Host "$Msg" -ForegroundColor Green }
function Show-Info    { param([string]$Msg); Write-Host "$Msg" -ForegroundColor Gray }
function Show-Bold    { param([string]$Msg); Write-Host "$Msg" -ForegroundColor White }

$Arch = $env:PROCESSOR_ARCHITECTURE
$Target = ""

if ($Arch -eq "AMD64") {
    $Target = "windows-amd64"
} elseif ($Arch -eq "ARM64") {
    $Target = "windows-arm64"
} else {
    Show-Error "Unsupported architecture: $Arch"
    exit 1
}

$GitHubOrg = "trywpm"
$Repo = "cli"
$ExeName = "wpm.exe"

$InstallDir = Join-Path $env:LOCALAPPDATA "wpm"
$ExePath = Join-Path $InstallDir $ExeName

$BaseUrl = "https://github.com/$GitHubOrg/$Repo/releases"
$BinaryName = "wpm-$Target.exe"

if ([string]::IsNullOrEmpty($Version)) {
    $Uri = "$BaseUrl/latest/download/$BinaryName"
} else {
    $Uri = "$BaseUrl/download/$Version/$BinaryName"
}

try {
    if (Get-Process "wpm" -ErrorAction SilentlyContinue) {
        Show-Error "wpm is currently running. Please close it and try again."
        exit 1
    }

    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    }

    Show-Info "Installing wpm for $Target..."
    Show-Info "Downloading from $Uri..."

    $TempFile = Join-Path $env:TEMP $BinaryName
    Invoke-WebRequest -Uri $Uri -OutFile $TempFile

    # Download Checksum
    $ChecksumUri = "$Uri.sha256"
    $TempChecksum = "$TempFile.sha256"

    try {
        Invoke-WebRequest -Uri $ChecksumUri -OutFile $TempChecksum -ErrorAction Stop

        Show-Info "Verifying checksum..."

        $ExpectedHash = (Get-Content $TempChecksum).Split(" ")[0].Trim()
        $ActualHash = (Get-FileHash -Path $TempFile -Algorithm SHA256).Hash

        if ($ExpectedHash -ne $ActualHash) {
            throw "Checksum mismatch! Expected: $ExpectedHash, Actual: $ActualHash"
        }
    } catch {
        Show-Error "Failed to download or verify checksum: $_"
        Remove-Item $TempFile -ErrorAction SilentlyContinue
        if (Test-Path $TempChecksum) { Remove-Item $TempChecksum -ErrorAction SilentlyContinue }
        exit 1
    }

    Move-Item -Path $TempFile -Destination $ExePath -Force

    if (Test-Path $TempChecksum) { Remove-Item $TempChecksum }

    $UserPathArgs = "Path", "User"
    $CurrentPath = [Environment]::GetEnvironmentVariable($UserPathArgs)

    if (($CurrentPath -split ';') -notcontains $InstallDir) {
        Show-Info "Adding $InstallDir to User PATH..."

        # Add to persistent Registry PATH
        [Environment]::SetEnvironmentVariable("Path", "$CurrentPath;$InstallDir", "User")

        $env:Path += ";$InstallDir"
    }

    Write-Host ""
    Show-Success "wpm installed to $ExePath"

    Write-Host ""
    Show-Info "To get started, run:"
    Write-Host ""
    Show-Bold "  wpm --help"

    Write-Host ""
    Show-Info "Note: You may need to restart your terminal for the PATH changes to take effect."
} catch {
    Show-Error $_.Exception.Message
    exit 1
}
