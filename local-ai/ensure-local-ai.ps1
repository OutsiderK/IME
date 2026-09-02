$ErrorActionPreference = 'Stop'

$healthUrl = 'http://127.0.0.1:8080/health'
$modelPath = Join-Path $PSScriptRoot 'models\Qwen3.5-4B-Q4_K_M.gguf'
$logDir = Join-Path $PSScriptRoot 'logs'
$wingetRoot = Join-Path $env:LOCALAPPDATA 'Microsoft\WinGet\Packages'

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
    exit 0
}

$serverPath = Get-ChildItem -LiteralPath $wingetRoot -Filter 'llama-server.exe' -Recurse -ErrorAction SilentlyContinue |
    Where-Object { $_.FullName -like '*ggml.llamacpp*' } |
    Select-Object -First 1 -ExpandProperty FullName

if (-not $serverPath) {
    throw 'llama-server.exe was not found. Run: winget install --id ggml.llamacpp --exact'
}

if (-not (Test-Path -LiteralPath $modelPath)) {
    throw "Model file was not found: $modelPath"
}

New-Item -ItemType Directory -Force -Path $logDir | Out-Null

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

Start-Process -FilePath $serverPath `
    -ArgumentList $serverArgs `
    -WindowStyle Hidden `
    -RedirectStandardOutput (Join-Path $logDir 'llama-server.stdout.log') `
    -RedirectStandardError (Join-Path $logDir 'llama-server.stderr.log')

for ($attempt = 0; $attempt -lt 40; $attempt++) {
    if (Test-LocalAI) {
        exit 0
    }
    Start-Sleep -Milliseconds 500
}

throw 'Local AI did not become healthy within 20 seconds.'

