# Usage:
#   powershell -ExecutionPolicy ByPass -File install.ps1
#   powershell -ExecutionPolicy ByPass -File install.ps1 -Version v1.0.0

param (
    [string]$Version
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

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
# Install to %LocalAppData%\wpm (Standard user-level location)
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
    Show-Info "Installing wpm for $Target..."

    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    }

    Show-Info "Downloading from $Uri..."
    $WebClient = New-Object System.Net.WebClient
    $WebClient.DownloadFile($Uri, $ExePath)

    # Check if InstallDir is in the User's persistent Path
    $UserPathArgs = "Path", "User"
    $CurrentPath = [Environment]::GetEnvironmentVariable($UserPathArgs)

    if ($CurrentPath -notlike "*$InstallDir*") {
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

} catch {
    Show-Error $_.Exception.Message
    exit 1
}
