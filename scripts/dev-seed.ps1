#Requires -Version 5.1
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot

Get-Content "$Root/fixtures/seed/postgres.sql" | kubectl -n qllm-dev exec -i deploy/postgres -- psql -U qllm -d crm
Get-Content "$Root/fixtures/seed/mysql.sql" | kubectl -n qllm-dev exec -i deploy/mysql -- mysql -uqllm -pqllm
Get-Content "$Root/fixtures/seed/mongo.js" | kubectl -n qllm-dev exec -i deploy/mongodb -- mongosh --quiet
Write-Host "seeds applied"
