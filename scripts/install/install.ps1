#!/usr/bin/env pwsh

param (
    [string]$Version
)

Set-StrictMode -Version Latest

$ProgressPreference = 'SilentlyContinue'
$ErrorActionPreference = 'Stop'

[Net.ServicePointManager]::SecurityProtocol = `
    [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

function Show-Info { param([string]$Msg); Write-Host $Msg -ForegroundColor Gray }
function Show-Bold { param([string]$Msg); Write-Host $Msg -ForegroundColor White }
function Show-Error { param([string]$Msg); Write-Host "error: $Msg" -ForegroundColor Red }
function Show-Success { param([string]$Msg); Write-Host $Msg -ForegroundColor Green }
function Show-Warning { param([string]$Msg); Write-Host "warning: $Msg" -ForegroundColor Yellow }

$arch = try {
    (Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Environment' -Name PROCESSOR_ARCHITECTURE -ErrorAction Stop).PROCESSOR_ARCHITECTURE
}
catch {
    if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
}

switch ($arch) {
    'AMD64' { $target = 'windows-amd64' }
    'ARM64' { $target = 'windows-arm64' }
    default {
        Show-Error "Unsupported architecture: $arch"
        exit 1
    }
}

$ExeName = 'wpm.exe'
$BinaryName = "wpm-$target.exe"

$InstallDir = if ($env:WPM_INSTALL) { $env:WPM_INSTALL } else { Join-Path $HOME '.wpm' }
$BinDir = Join-Path $InstallDir 'bin'
$CompletionsDir = Join-Path $InstallDir 'completions'
$ExePath = Join-Path $BinDir $ExeName

$BaseUrl = 'https://github.com/trywpm/cli/releases'
$Uri = if ([string]::IsNullOrEmpty($Version)) {
    "$BaseUrl/latest/download/$BinaryName"
}
else {
    "$BaseUrl/download/$Version/$BinaryName"
}

# remove any old wpm.exe left by the legacy installer.
$LegacyExePath = Join-Path $env:LOCALAPPDATA 'wpm\wpm.exe'
if (Test-Path $LegacyExePath) {
    Show-Info "A previous installation of wpm was found in $env:LOCALAPPDATA\wpm."
    Show-Info 'Removing it to avoid conflicts...'
    Remove-Item -Path $LegacyExePath -Force -ErrorAction SilentlyContinue
    Show-Success "Removed $LegacyExePath"
}

try {
    if (Get-Process -Name 'wpm' -ErrorAction SilentlyContinue) {
        Show-Error 'wpm is currently running. Please close it and try again.'
        exit 1
    }

    foreach ($dir in @($BinDir, $CompletionsDir)) {
        if (-not (Test-Path -Path $dir)) {
            New-Item -ItemType Directory -Force -Path $dir | Out-Null
        }
    }

    $TempFile = Join-Path $env:TEMP $BinaryName
    $TempChecksum = "$TempFile.sha256"

    Show-Info 'Downloading wpm...'
    try {
        Invoke-WebRequest -Uri $Uri -OutFile $TempFile -UseBasicParsing
    }
    catch {
        Show-Error "Failed to download wpm from `"$Uri`": $($_.Exception.Message)"
        exit 1
    }

    # checksum verification.
    $haveChecksum = $false
    try {
        Invoke-WebRequest -Uri "$Uri.sha256" -OutFile $TempChecksum -UseBasicParsing -ErrorAction Stop
        $haveChecksum = $true
    }
    catch {}

    if ($haveChecksum) {
        Show-Info 'Verifying checksum...'
        $expected = ((Get-Content -Path $TempChecksum -Raw).Trim() -split '\s+')[0]
        $actual = (Get-FileHash -Path $TempFile -Algorithm SHA256).Hash

        if ($expected -ne $actual) {
            Remove-Item -Path $TempFile, $TempChecksum -ErrorAction SilentlyContinue
            Show-Error "Checksum mismatch! expected $expected, got $actual"
            exit 1
        }

        Remove-Item -Path $TempChecksum -ErrorAction SilentlyContinue
    }

    Move-Item -Path $TempFile -Destination $ExePath -Force

    # shell completion setup.
    $CompletionFile = Join-Path $CompletionsDir 'wpm.ps1'
    try {
        $completion = & $ExePath completion powershell 2>$null
        if ($LASTEXITCODE -eq 0 -and $completion) {
            $completion | Out-File -FilePath $CompletionFile -Encoding utf8
        }
    }
    catch {}

    Show-Success "wpm installed to $ExePath"

    $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $PathEntries = if ($UserPath) { $UserPath -split ';' | ForEach-Object { $_.TrimEnd('\') } } else { @() }
    $BinOnUserPath = $PathEntries -contains $BinDir

    if (-not $BinOnUserPath) {
        $NewPath = if ([string]::IsNullOrEmpty($UserPath)) { $BinDir } else { "$UserPath;$BinDir" }
        [Environment]::SetEnvironmentVariable('Path', $NewPath, 'User')

        $env:Path = "$env:Path;$BinDir"

        Show-Info "Added `"$BinDir`" to your system `$PATH"
    }

    $ProfilePath = $PROFILE.CurrentUserCurrentHost
    $ProfileDir = Split-Path -Path $ProfilePath -Parent
    if (-not (Test-Path -Path $ProfileDir)) {
        New-Item -ItemType Directory -Path $ProfileDir -Force | Out-Null
    }

    $marker = '# wpm completions'
    $existing = if (Test-Path -Path $ProfilePath) {
        Get-Content -Path $ProfilePath -Raw
    }
    else { '' }

    if (-not $existing -or ($existing -notmatch [regex]::Escape($marker))) {
        if ($InstallDir.StartsWith("$HOME\") -or $InstallDir -eq $HOME) {
            $InstallRef = '"$HOME' + $InstallDir.Substring($HOME.Length) + '"'
        }
        else {
            $escaped = $InstallDir -replace "'", "''"
            $InstallRef = "'$escaped'"
        }

        $block = @"

$marker
`$env:WPM_INSTALL = $InstallRef
if (Test-Path "`$env:WPM_INSTALL\completions\wpm.ps1") { . "`$env:WPM_INSTALL\completions\wpm.ps1" }
"@
        Add-Content -Path $ProfilePath -Value $block -Encoding utf8

        Show-Info "Added completions to `"$ProfilePath`""
    }

    Write-Host ''
    Show-Info 'To get started, run:'
    Write-Host ''
    Show-Bold "  . `$PROFILE"
    Show-Bold '  wpm --help'
}
catch {
    Show-Error $_.Exception.Message
    exit 1
}
