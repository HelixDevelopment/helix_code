#!/usr/bin/env pwsh
<#
.SYNOPSIS
    HelixCode Windows Installer
.DESCRIPTION
    Downloads and installs HelixCode on Windows
.PARAMETER Version
    Version to install (default: latest)
.PARAMETER InstallDir
    Installation directory (default: $env:LOCALAPPDATA\HelixCode)
#>

param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\HelixCode"
)

$ErrorActionPreference = "Stop"

Write-Host "Installing HelixCode $Version..." -ForegroundColor Green

$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }

$url = "https://github.com/helixcode/helixcode/releases/download/v$Version/helixcode-$Version.windows.$arch.zip"

Write-Host "Downloading from $url..." -ForegroundColor Yellow
$zipPath = "$env:TEMP\helixcode.zip"
Invoke-WebRequest -Uri $url -OutFile $zipPath
Expand-Archive -Path $zipPath -DestinationPath $InstallDir -Force

$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($userPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$userPath;$InstallDir", "User")
    $env:PATH += ";$InstallDir"
}

Write-Host "HelixCode installed to $InstallDir" -ForegroundColor Green
Write-Host "Run 'helixcode version' to verify." -ForegroundColor Green
