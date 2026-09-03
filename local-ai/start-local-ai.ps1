$ErrorActionPreference = 'Stop'

$modelPath = Join-Path $env:LOCALAPPDATA 'MoqiAI\models\Qwen3.5-4B-Q4_K_M.gguf'
$wingetRoot = Join-Path $env:LOCALAPPDATA 'Microsoft\WinGet\Packages'
$serverPath = Get-ChildItem -LiteralPath $wingetRoot -Filter 'llama-server.exe' -Recurse -ErrorAction SilentlyContinue |
    Where-Object { $_.FullName -like '*ggml.llamacpp*' } |
    Select-Object -First 1 -ExpandProperty FullName

if (-not $serverPath) {
    throw 'llama-server.exe was not found. Run: winget install --id ggml.llamacpp --exact'
}

if (-not (Test-Path -LiteralPath $modelPath)) {
    throw "Model file was not found: $modelPath"
}

Write-Host 'Starting local AI...' -ForegroundColor Cyan
Write-Host 'API: http://127.0.0.1:8080/v1'
Write-Host 'Model: qwen3.5-4b-ime'
Write-Host 'API key: ime-local'
Write-Host 'Press Ctrl+C to stop.'

& $serverPath `
    --model $modelPath `
    --alias 'qwen3.5-4b-ime' `
    --host '127.0.0.1' `
    --port 8080 `
    --ctx-size 4096 `
    --predict 32 `
    --n-gpu-layers all `
    --flash-attn on `
    --parallel 1 `
    --reasoning off `
    --api-key 'ime-local' `
    --cors-origins 'localhost'

exit $LASTEXITCODE
