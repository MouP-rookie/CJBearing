$ErrorActionPreference = "Stop"
Push-Location "$PSScriptRoot\.."
try {
  npm.cmd install
  npm.cmd run build
  Push-Location server
  go test ./...
  go build -buildvcs=false -o bin\bearing-inspection.exe ./cmd/api
  go build -buildvcs=false -o bin\qms.exe ./cmd/qms
  Pop-Location
} finally { Pop-Location }
