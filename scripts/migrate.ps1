param([string]$Command = "up")
$ErrorActionPreference = "Stop"
Push-Location "$PSScriptRoot\..\server"
try { go run ./cmd/migrate $Command } finally { Pop-Location }