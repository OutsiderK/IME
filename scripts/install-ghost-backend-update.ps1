$ErrorActionPreference = 'Stop'

$resultPath = Join-Path $env:LOCALAPPDATA 'Temp\moqi-backend-update-result.log'
$stageServer = Join-Path $env:LOCALAPPDATA 'Temp\moqi-ghost-stage\backend\server.exe'
$installRoot = 'C:\Program Files (x86)\MoqiIM'
$launcherPath = Join-Path $installRoot 'MoqiLauncher.exe'
$serverPath = Join-Path $installRoot 'moqi-ime\server.exe'

try {
    Get-Process MoqiLauncher, server -ErrorAction SilentlyContinue |
        Stop-Process -Force -ErrorAction SilentlyContinue

    for ($attempt = 0; $attempt -lt 40; $attempt++) {
        if (-not (Get-Process server -ErrorAction SilentlyContinue)) { break }
        Start-Sleep -Milliseconds 100
    }

    $copied = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        try {
            Copy-Item -LiteralPath $stageServer -Destination $serverPath -Force
            $copied = $true
            break
        } catch {
            Start-Sleep -Milliseconds 100
        }
    }
    if (-not $copied) { throw 'server.exe remained locked after retries' }

    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $stageServer).Hash -ne
        (Get-FileHash -Algorithm SHA256 -LiteralPath $serverPath).Hash) {
        throw 'Installed backend hash mismatch'
    }

    Start-Process -FilePath $launcherPath -WorkingDirectory $installRoot -WindowStyle Hidden
    'OK' | Set-Content -LiteralPath $resultPath -Encoding UTF8
} catch {
    ("FAILED`r`n" + ($_ | Out-String)) | Set-Content -LiteralPath $resultPath -Encoding UTF8
    exit 1
}
