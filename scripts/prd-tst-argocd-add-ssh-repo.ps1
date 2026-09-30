# Registers an SSH git repo for Argo CD (in-cluster). git-gui/ssh-agent on Windows is NOT visible to Argo pods.
param(
    [string]$KeyPath = "",
    [string]$RepoSsh = "git@github.com:jpandrade30/qLLM.git"
)
$ErrorActionPreference = "Stop"
if (-not $KeyPath) {
    foreach ($c in @(
            (Join-Path $env:USERPROFILE ".ssh\id_ed25519"),
            (Join-Path $env:USERPROFILE ".ssh\id_rsa")
        )) {
        if (Test-Path $c) { $KeyPath = $c; break }
    }
}
if (-not $KeyPath -or -not (Test-Path $KeyPath)) {
    throw "Private key not found. Pass -KeyPath to the OpenSSH private key (not .pub, not PuTTY .ppk)."
}
$raw = Get-Content -LiteralPath $KeyPath -Raw
if ($raw -match "PuTTY-User-Key-File") {
    throw "This is a PuTTY .ppk. In PuTTYgen: Conversions -> Export OpenSSH key, then -KeyPath that file."
}
if ($raw -notmatch "BEGIN .*PRIVATE KEY") {
    throw "File does not look like an OpenSSH private key (do not use .pub)."
}

kubectl create secret generic repo-qllm-ssh `
    --namespace argocd `
    --from-literal=type=git `
    --from-literal=url=$RepoSsh `
    --from-file=sshPrivateKey=$KeyPath `
    --dry-run=client -o yaml |
    kubectl label --local -f - argocd.argoproj.io/secret-type=repository -o yaml |
    kubectl apply -f -

$patch = @{ spec = @{ source = @{ repoURL = $RepoSsh } } } | ConvertTo-Json -Compress
$tmp = Join-Path $env:TEMP "argo-repo-url.json"
[System.IO.File]::WriteAllText($tmp, $patch)
kubectl -n argocd patch application qllm-prd-sim --type merge --patch-file $tmp
Remove-Item $tmp -ErrorAction SilentlyContinue

Write-Host "Secret repo-qllm-ssh created. Application repoURL -> $RepoSsh"
Write-Host "In Argo UI: delete any broken SSH repo (the one that had no key), then Refresh the app."
