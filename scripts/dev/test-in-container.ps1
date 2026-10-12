#Requires -Version 5.1
# Run Go unit tests (pure + -tags duckdb) inside Linux via nerdctl.
# Usage:
#   .\scripts\dev\test-in-container.ps1
#   .\scripts\dev\test-in-container.ps1 -FailBuild   # --target test (no image tag)
#   .\scripts\dev\test-in-container.ps1 -Remount     # mount repo after image build
param(
    [switch]$FailBuild,
    [switch]$Remount
)
$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Set-Location $Root

if ($FailBuild) {
    Write-Host "nerdctl build -f Dockerfile.test --target test ." -ForegroundColor DarkGray
    nerdctl build -f Dockerfile.test --target test .
    exit $LASTEXITCODE
}

Write-Host "nerdctl build -f Dockerfile.test -t qllm-test ." -ForegroundColor DarkGray
nerdctl build -f Dockerfile.test -t qllm-test .
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# -t so ANSI colors from run-go-tests.sh render in the host terminal
if ($Remount) {
    Write-Host "nerdctl run --rm -t -v ${Root}:/src -w /src qllm-test" -ForegroundColor DarkGray
    nerdctl run --rm -t -v "${Root}:/src" -w /src qllm-test
} else {
    Write-Host "nerdctl run --rm -t qllm-test" -ForegroundColor DarkGray
    nerdctl run --rm -t qllm-test
}
exit $LASTEXITCODE
