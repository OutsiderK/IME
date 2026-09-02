$ErrorActionPreference = 'Stop'
$resultPath = 'C:\Users\joeyK\AppData\Local\Temp\moqi-install-result.log'
trap {
    ("FAILED`r`n" + ($_ | Out-String)) | Set-Content -LiteralPath $resultPath -Encoding UTF8
    exit 1
}

$projectRoot = 'C:\Users\joeyK\OneDrive\桌面\Projects\U输入法'
$installRoot = 'C:\Program Files (x86)\MoqiIM'
$launcherPath = Join-Path $installRoot 'MoqiLauncher.exe'
$serverPath = Join-Path $installRoot 'moqi-ime\server.exe'

foreach ($process in Get-Process -ErrorAction SilentlyContinue) {
    if ($process.ProcessName -notin @('MoqiLauncher', 'server')) {
        continue
    }
    $path = $process.Path
    if ([string]::Equals($path, $launcherPath, [System.StringComparison]::OrdinalIgnoreCase) -or
        [string]::Equals($path, $serverPath, [System.StringComparison]::OrdinalIgnoreCase)) {
        Stop-Process -Id $process.Id -Force
    }
}

$packageRoot = Join-Path $projectRoot 'build\ghost-completion'
$configSource = 'C:\Users\joeyK\OneDrive\文档\ChatGPT\输入法\local-ai\moqi\ai_config.json'

Copy-Item -LiteralPath (Join-Path $packageRoot 'x86\MoqiTextService.dll') -Destination (Join-Path $installRoot 'MoqiTextService.dll') -Force
Copy-Item -LiteralPath (Join-Path $packageRoot 'x64\MoqiTextService.dll') -Destination (Join-Path $installRoot 'x64\MoqiTextService.dll') -Force
Copy-Item -LiteralPath (Join-Path $packageRoot 'backend\server.exe') -Destination $serverPath -Force
Copy-Item -LiteralPath $configSource -Destination (Join-Path $installRoot 'moqi-ime\input_methods\rime\ai_config.json') -Force

$userConfigRoot = 'C:\Users\joeyK\AppData\Roaming\Moqi'
New-Item -ItemType Directory -Force -Path $userConfigRoot | Out-Null
Copy-Item -LiteralPath $configSource -Destination (Join-Path $userConfigRoot 'ai_config.json') -Force

Start-Process -FilePath $launcherPath -WorkingDirectory $installRoot -WindowStyle Hidden
'OK' | Set-Content -LiteralPath $resultPath -Encoding UTF8
