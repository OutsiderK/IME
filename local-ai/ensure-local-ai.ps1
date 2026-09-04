param(
    [string] $ModelRoot = (Join-Path $env:LOCALAPPDATA 'MoqiAI\models'),
    [ValidateRange(0, 120)]
    [int] $ResourceWaitSeconds = 45
)

$ErrorActionPreference = 'Stop'

$healthUrl = 'http://127.0.0.1:8080/health'
$modelPath = Join-Path $ModelRoot 'Qwen3.5-4B-Q4_K_M.gguf'
$runtimeRoot = Split-Path -Parent $ModelRoot
$logDir = Join-Path $runtimeRoot 'logs'
$startupLog = Join-Path $logDir 'startup.log'
$wingetRoot = Join-Path $env:LOCALAPPDATA 'Microsoft\WinGet\Packages'

New-Item -ItemType Directory -Force -Path $logDir | Out-Null

function Write-StartupLog {
    param([Parameter(Mandatory = $true)][string] $Message)
    try {
        $timestamp = [DateTime]::Now.ToString('yyyy-MM-dd HH:mm:ss.fff')
        Add-Content -LiteralPath $startupLog -Value "[$timestamp] $Message" -Encoding UTF8
    }
    catch {
        # Diagnostics must never prevent the model service from starting.
    }
}

function Test-LocalAI {
    try {
        $response = Invoke-RestMethod -Uri $healthUrl -TimeoutSec 1
        return $response.status -eq 'ok'
    }
    catch {
        return $false
    }
}

if (Test-LocalAI) {
    Write-StartupLog 'Local AI is already healthy.'
    Write-Output '0'
    exit 0
}

$serverPath = $null
$waitDeadline = [DateTime]::UtcNow.AddSeconds($ResourceWaitSeconds)
do {
    if (Test-LocalAI) {
        Write-StartupLog 'Local AI became healthy while waiting for startup resources.'
        Write-Output '0'
        exit 0
    }
    $serverPath = Get-ChildItem -LiteralPath $wingetRoot -Filter 'llama-server.exe' -Recurse -ErrorAction SilentlyContinue |
        Where-Object { $_.FullName -like '*ggml.llamacpp*' } |
        Select-Object -First 1 -ExpandProperty FullName
    if ($serverPath -and (Test-Path -LiteralPath $modelPath -PathType Leaf)) {
        break
    }
    Start-Sleep -Milliseconds 500
} while ([DateTime]::UtcNow -lt $waitDeadline)

if (-not $serverPath) {
    Write-StartupLog "llama-server.exe was not found under $wingetRoot."
    throw 'llama-server.exe was not found. Run: winget install --id ggml.llamacpp --exact'
}

if (-not (Test-Path -LiteralPath $modelPath -PathType Leaf)) {
    Write-StartupLog "Model file was not found: $modelPath"
    throw "Model file was not found: $modelPath"
}

Write-StartupLog "Starting local AI with model $modelPath"

$serverArgs = @(
    '--model', $modelPath,
    '--alias', 'qwen3.5-4b-ime',
    '--host', '127.0.0.1',
    '--port', '8080',
    '--ctx-size', '4096',
    '--predict', '32',
    '--n-gpu-layers', 'all',
    '--flash-attn', 'on',
    '--parallel', '1',
    '--reasoning', 'off',
    '--api-key', 'ime-local',
    '--cors-origins', 'localhost'
)

$process = Start-Process -FilePath $serverPath `
    -ArgumentList $serverArgs `
    -WindowStyle Hidden `
    -RedirectStandardOutput (Join-Path $logDir 'llama-server.stdout.log') `
    -RedirectStandardError (Join-Path $logDir 'llama-server.stderr.log') `
    -PassThru

for ($attempt = 0; $attempt -lt 40; $attempt++) {
    if (Test-LocalAI) {
        Write-StartupLog "Local AI became healthy (PID $($process.Id))."
        Write-Output $process.Id
        exit 0
    }
    Start-Sleep -Milliseconds 500
}

Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
Write-StartupLog "Local AI did not become healthy within 20 seconds (PID $($process.Id))."
throw 'Local AI did not become healthy within 20 seconds.'
