#!/usr/bin/env pwsh
# Build script for Windows - outputs to repo bin/ directory

$ErrorActionPreference = "Stop"
$binDir = Join-Path $PSScriptRoot "..\bin"
$outPath = Join-Path $binDir "repo-context.exe"

Write-Host "Building Repo Context CLI (Go)" -ForegroundColor Cyan
Write-Host ""

$goCommand = Get-Command go -ErrorAction SilentlyContinue
if (-not $goCommand) {
    Write-Error "Go is not installed or not in PATH. Please install Go from https://golang.org/dl/"
    exit 1
}

$goVersion = & go version
Write-Host "Go installed: $goVersion" -ForegroundColor Green

if (-not (Test-Path $binDir)) {
    New-Item -ItemType Directory -Path $binDir -Force | Out-Null
}

Write-Host "Building executable..." -ForegroundColor Cyan
go build -ldflags="-s -w" -o $outPath

if ($LASTEXITCODE -eq 0) {
    $size = (Get-Item $outPath).Length / 1MB
    Write-Host ""
    Write-Host "Build successful!" -ForegroundColor Green
    Write-Host "   Size: $([math]::Round($size, 2)) MB" -ForegroundColor White
    Write-Host "   Output: $outPath" -ForegroundColor White
    Write-Host ""
    Write-Host "Test it:" -ForegroundColor Cyan
    Write-Host "  repo-context help" -ForegroundColor White
    Write-Host "  repo-context version" -ForegroundColor White
    Write-Host ""
} else {
    Write-Error "Build failed!"
    exit 1
}
