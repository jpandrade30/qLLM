@echo off
REM Double-click or run from Explorer: opens PowerShell with qLLM/DuckDB env.
cd /d "%~dp0\..\.."
powershell.exe -NoLogo -NoExit -ExecutionPolicy Bypass -File "%~dp0dev-shell.ps1"
