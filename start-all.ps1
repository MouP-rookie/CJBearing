# 启动本机开发环境：PostgreSQL + Go API + QMS + 三个前端（PC / PDA / 门户）
# 用法：双击项目根目录的「启动项目.cmd」，或在 PowerShell 里执行本脚本
$ErrorActionPreference = 'Stop'
$repo = $PSScriptRoot
$server = Join-Path $repo 'server'
$run = Join-Path $repo '.run'
$bin = Join-Path $run 'bin'
$pgctl = 'D:\PostgreSQL\18.6\pgsql\bin\pg_ctl.exe'
$pgdata = 'D:\PostgreSQL\data'
New-Item -ItemType Directory -Force -Path $bin | Out-Null

function Test-Port([int]$port) {
    $null -ne (Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue)
}

# 1) PostgreSQL
& $pgctl status -D $pgdata *> $null
if ($LASTEXITCODE -ne 0) {
    Write-Host '启动 PostgreSQL ...'
    & $pgctl start -D $pgdata -l 'D:\PostgreSQL\postgresql.log' -w -t 60 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL 启动失败' }
} else {
    Write-Host 'PostgreSQL 已在运行'
}

# 2) 编译后端
Write-Host '编译 API / QMS ...'
Push-Location $server
try {
    $env:GOFLAGS = '-buildvcs=false'
    go run ./cmd/migrate up
    if ($LASTEXITCODE -ne 0) { throw '数据库迁移失败' }
    go run ./cmd/seed
    if ($LASTEXITCODE -ne 0) { throw '初始化基础数据失败' }
    go build -o (Join-Path $bin 'api.exe') ./cmd/api
    if ($LASTEXITCODE -ne 0) { throw 'API 编译失败' }
    go build -o (Join-Path $bin 'qms.exe') ./cmd/qms
    if ($LASTEXITCODE -ne 0) { throw 'QMS 编译失败' }
} finally {
    Pop-Location
}

# 3) 启动后端服务
if (-not (Test-Port 8090)) {
    Write-Host '启动 API ...'
    Start-Process -FilePath (Join-Path $bin 'api.exe') -WorkingDirectory $server -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $run 'api.log') -RedirectStandardError (Join-Path $run 'api.err.log')
} else {
    Write-Host 'API 已在运行'
}
if (-not (Test-Port 8081)) {
    Write-Host '启动 QMS ...'
    Start-Process -FilePath (Join-Path $bin 'qms.exe') -WorkingDirectory $server -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $run 'qms.log') -RedirectStandardError (Join-Path $run 'qms.err.log')
} else {
    Write-Host 'QMS 已在运行'
}

# 4) 启动前端（机器上没装 node 时，用 DSH 自带的 node）
$node = (Get-Command node -ErrorAction SilentlyContinue).Source
if (-not $node) {
    $candidate = Join-Path $env:USERPROFILE '.dsh\dsh-runtimes\dsh-next-primary-runtime\dependencies\node\bin\node.exe'
    if (Test-Path $candidate) { $node = $candidate }
}
$vite = Join-Path $repo 'node_modules\vite\bin\vite.js'
foreach ($w in @(@('web-pc', 5173), @('web-pda', 5174), @('web-portal', 5175))) {
    $dir = Join-Path $repo $w[0]
    if (Test-Port $w[1]) { Write-Host "$($w[0]) 已在运行"; continue }
    if (-not $node) { Write-Warning "找不到 node，跳过 $($w[0])"; continue }
    Write-Host "启动 $($w[0]) ..."
    Start-Process -FilePath $node -ArgumentList @($vite, '--host', '0.0.0.0') -WorkingDirectory $dir -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $run ($w[0] + '.log')) -RedirectStandardError (Join-Path $run ($w[0] + '.err.log'))
}

Start-Sleep -Seconds 3
Write-Host ''
Write-Host '========================================'
Write-Host ' 统一门户： http://127.0.0.1:5175/portal/'
Write-Host ' PC 端：    http://127.0.0.1:5173/pc/'
Write-Host ' PDA 端：   http://127.0.0.1:5174/pda/'
Write-Host ' API：      http://127.0.0.1:8090'
Write-Host ' QMS：      http://127.0.0.1:8081'
Write-Host ' 管理员：   admin / 1234'
Write-Host '========================================'
Write-Host ''
Write-Host '日志目录：' $run
