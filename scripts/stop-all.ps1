$ErrorActionPreference = "Stop"
$ports = @(8090, 8081, 5173, 5174, 5175)
foreach ($port in $ports) {
    $connections = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
    foreach ($connection in $connections) {
        if ($connection.OwningProcess -gt 0) {
            Stop-Process -Id $connection.OwningProcess -Force -ErrorAction SilentlyContinue
        }
    }
}
$runDir = Join-Path $PSScriptRoot "..\.run"
if (Test-Path -LiteralPath $runDir) {
    Remove-Item -LiteralPath $runDir -Recurse -Force
}
Write-Host "API, QMS, portal, PC and PDA services stopped."
