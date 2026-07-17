#Requires -Version 5.1
$ErrorActionPreference = "Stop"

Start-Process -NoNewWindow kubectl -ArgumentList @("-n","qllm-dev","port-forward","svc/postgres","5432:5432") 
Start-Process -NoNewWindow kubectl -ArgumentList @("-n","qllm-dev","port-forward","svc/mysql","3306:3306")
Start-Process -NoNewWindow kubectl -ArgumentList @("-n","qllm-dev","port-forward","svc/mongodb","27017:27017")
Start-Process -NoNewWindow kubectl -ArgumentList @("-n","qllm-dev","port-forward","svc/test-api","18080:8080")
Start-Sleep -Seconds 2

Write-Host @"
`$env:QLLM_CRM_PG_HOST='127.0.0.1'
`$env:QLLM_CRM_PG_USER='qllm'
`$env:QLLM_CRM_PG_PASSWORD='qllm'
`$env:QLLM_BILLING_MYSQL_HOST='127.0.0.1'
`$env:QLLM_BILLING_MYSQL_USER='qllm'
`$env:QLLM_BILLING_MYSQL_PASSWORD='qllm'
`$env:QLLM_EVENTS_MONGO_URI='mongodb://127.0.0.1:27017'
`$env:QLLM_LEGACY_API_BASE_URL='http://127.0.0.1:18080'
"@
Write-Host "port-forwards started (pg 5432, mysql 3306, mongo 27017, api 18080)" -ForegroundColor Yellow
