$ErrorActionPreference = "Stop"
$server = (Resolve-Path "$PSScriptRoot\..\server").Path
Set-Location $server
$env:GOFLAGS = "-buildvcs=false"
go run ./cmd/dev