#Requires -Version 5.1
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$SeedDir = Join-Path $Root "fixtures\seed"
$VenvDir = Join-Path $SeedDir ".venv"
$VenvPy = Join-Path $VenvDir "Scripts\python.exe"

function Test-PythonHasPip {
    param([Parameter(Mandatory)][string]$Exe)
    if (-not (Test-Path $Exe)) { return $false }
    # Foreign/broken venvs on PATH often lack pip; never use those as host.
    $probe = & $Exe -c "import pip, sys; print(sys.executable)" 2>$null
    return ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($probe))
}

function Resolve-HostPython {
    $candidates = [System.Collections.Generic.List[string]]::new()

    # Prefer known good installs over PATH (PATH often hits foreign broken .venvs).
    $uvRoot = Join-Path $env:APPDATA "uv\python"
    if (Test-Path $uvRoot) {
        Get-ChildItem $uvRoot -Directory -ErrorAction SilentlyContinue |
            Sort-Object Name -Descending |
            ForEach-Object {
                $uvPy = Join-Path $_.FullName "python.exe"
                if (Test-Path $uvPy) { $candidates.Add($uvPy) }
            }
    }

    foreach ($hint in @(
            "$env:LOCALAPPDATA\Programs\Python\Python313\python.exe",
            "$env:LOCALAPPDATA\Programs\Python\Python312\python.exe",
            "$env:LOCALAPPDATA\Programs\Python\Python311\python.exe",
            "C:\Python313\python.exe",
            "C:\Python312\python.exe"
        )) {
        if (Test-Path $hint) { $candidates.Add($hint) }
    }

    $pyLauncher = Get-Command py -ErrorAction SilentlyContinue
    if ($pyLauncher) {
        try {
            $fromPy = & py -3 -c "import sys; print(sys.executable)" 2>$null
            if ($LASTEXITCODE -eq 0 -and $fromPy) { $candidates.Add($fromPy.Trim()) }
        } catch {}
    }

    foreach ($name in @("python3", "python")) {
        $cmd = Get-Command $name -ErrorAction SilentlyContinue
        if ($cmd) { $candidates.Add($cmd.Source) }
    }

    $seen = @{}
    foreach ($exe in $candidates) {
        if ([string]::IsNullOrWhiteSpace($exe)) { continue }
        $key = $exe.ToLowerInvariant()
        if ($seen.ContainsKey($key)) { continue }
        $seen[$key] = $true
        # Skip any .venv except our seed venv (PATH often points at foreign broken ones)
        if ($exe -match '\\\.venv\\' -and ($exe -ne $VenvPy)) { continue }
        if (Test-PythonHasPip $exe) {
            Write-Output $exe
            return
        }
    }
    throw "No usable Python with pip found. Install Python 3 (python.org) or: uv python install 3.13"
}

$needVenv = -not (Test-Path $VenvPy) -or -not (Test-PythonHasPip $VenvPy)
if ($needVenv) {
    if (Test-Path $VenvDir) {
        Write-Host "Recreating seed venv (missing or broken pip) ..."
        Remove-Item -Recurse -Force $VenvDir
    }
    $hostPy = Resolve-HostPython
    Write-Host "Creating seed venv with $hostPy ..."
    & $hostPy -m venv $VenvDir
    if (-not (Test-Path $VenvPy)) {
        throw "failed to create venv at $VenvDir"
    }
}

Write-Host "Using seed venv: $VenvPy"
& $VenvPy -m pip install --upgrade pip
if ($LASTEXITCODE -ne 0) { throw "pip upgrade failed in seed venv" }
& $VenvPy -m pip install -r (Join-Path $SeedDir "requirements.txt")
if ($LASTEXITCODE -ne 0) { throw "pip install requirements failed" }

$env:QLLM_CRM_PG_HOST = if ($env:QLLM_CRM_PG_HOST) { $env:QLLM_CRM_PG_HOST } else { "127.0.0.1" }
$env:QLLM_CRM_PG_USER = if ($env:QLLM_CRM_PG_USER) { $env:QLLM_CRM_PG_USER } else { "qllm" }
$env:QLLM_CRM_PG_PASSWORD = if ($env:QLLM_CRM_PG_PASSWORD) { $env:QLLM_CRM_PG_PASSWORD } else { "qllm" }
$env:QLLM_BILLING_MYSQL_HOST = if ($env:QLLM_BILLING_MYSQL_HOST) { $env:QLLM_BILLING_MYSQL_HOST } else { "127.0.0.1" }
$env:QLLM_BILLING_MYSQL_USER = if ($env:QLLM_BILLING_MYSQL_USER) { $env:QLLM_BILLING_MYSQL_USER } else { "qllm" }
$env:QLLM_BILLING_MYSQL_PASSWORD = if ($env:QLLM_BILLING_MYSQL_PASSWORD) { $env:QLLM_BILLING_MYSQL_PASSWORD } else { "qllm" }
$env:QLLM_EVENTS_MONGO_URI = if ($env:QLLM_EVENTS_MONGO_URI) { $env:QLLM_EVENTS_MONGO_URI } else { "mongodb://127.0.0.1:27017" }

$regenerate = $false
$customers = 40
foreach ($a in $args) {
    if ($a -eq "--regenerate") { $regenerate = $true }
    elseif ($a -match '^\d+$') { $customers = [int]$a }
}
if ($regenerate) {
    & $VenvPy (Join-Path $SeedDir "generate_dataset.py") --customers $customers --seed 42
    if ($LASTEXITCODE -ne 0) { throw "generate_dataset.py failed" }
}
& $VenvPy (Join-Path $SeedDir "load_dataset.py")
if ($LASTEXITCODE -ne 0) { throw "load_dataset.py failed" }
Write-Host "dataset loaded from fixtures/datasets/v1 (rebuild/restart compose test-api if data.json changed)"
