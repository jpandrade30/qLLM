# Create a slim qLLM project folder. Usage: .\scripts\standalone\init-standalone.ps1 --user Alice [--out DIR] [--force]
$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$script = Join-Path $PSScriptRoot "init-standalone.py"
$py = Get-Command python -ErrorAction SilentlyContinue
if (-not $py) {
    $py = Get-Command python3 -ErrorAction SilentlyContinue
}
if (-not $py) {
    throw "python or python3 is required"
}
& $py.Source $script @args
exit $LASTEXITCODE
