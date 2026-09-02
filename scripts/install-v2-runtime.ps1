$ErrorActionPreference = 'Stop'

$stage = 'C:\Users\joeyK\AppData\Local\Temp\moqi-register-v2'
$installRoot = 'C:\Program Files (x86)\MoqiIM'
$launcher = Join-Path $installRoot 'MoqiLauncher.exe'
$server = Join-Path $installRoot 'moqi-ime\server.exe'
$resultPath = 'C:\Users\joeyK\AppData\Local\Temp\moqi-install-v2-result.log'
$runMarker = [DateTime]::UtcNow.ToString('o')

try {
    $stagedServer = Join-Path $stage 'backend\server.exe'
    $stagedConfig = Join-Path $stage 'config\ai_config.json'
    $stagedDll64 = Join-Path $stage 'x64\MoqiTextService.dll'
    $stagedDll32 = Join-Path $stage 'x86\MoqiTextService.dll'
    foreach ($path in @($stagedServer, $stagedConfig, $stagedDll64, $stagedDll32)) {
        if (-not (Test-Path -LiteralPath $path)) {
            throw "Missing staged file: $path"
        }
    }

    foreach ($process in Get-Process -ErrorAction SilentlyContinue) {
        if ($process.ProcessName -notin @('MoqiLauncher', 'server')) {
            continue
        }
        if ([string]::Equals($process.Path, $launcher, [System.StringComparison]::OrdinalIgnoreCase) -or
            [string]::Equals($process.Path, $server, [System.StringComparison]::OrdinalIgnoreCase)) {
            Stop-Process -Id $process.Id -Force
        }
    }
    Start-Sleep -Milliseconds 800

    Copy-Item -LiteralPath $stagedServer -Destination $server -Force
    Copy-Item -LiteralPath $stagedConfig -Destination (Join-Path $installRoot 'moqi-ime\input_methods\rime\ai_config.json') -Force
    Copy-Item -LiteralPath $stagedDll32 -Destination (Join-Path $installRoot 'MoqiTextService.dll') -Force
    Copy-Item -LiteralPath $stagedDll64 -Destination (Join-Path $installRoot 'x64\MoqiTextService.dll') -Force

    $dll64 = Join-Path $installRoot 'x64\MoqiTextService.dll'
    $register64 = Start-Process -FilePath 'C:\Windows\System32\regsvr32.exe' `
        -ArgumentList ('/s "{0}"' -f $dll64) -Wait -PassThru -WindowStyle Hidden
    if ($register64.ExitCode -ne 0) {
        throw "64-bit regsvr32 failed: $($register64.ExitCode)"
    }
    $dll32 = Join-Path $installRoot 'MoqiTextService.dll'
    $register32 = Start-Process -FilePath 'C:\Windows\SysWOW64\regsvr32.exe' `
        -ArgumentList ('/s "{0}"' -f $dll32) -Wait -PassThru -WindowStyle Hidden
    if ($register32.ExitCode -ne 0) {
        throw "32-bit regsvr32 failed: $($register32.ExitCode)"
    }

    ("OK`r`n" + $runMarker) | Set-Content -LiteralPath $resultPath -Encoding UTF8
}
catch {
    ("FAILED`r`n" + $runMarker + "`r`n" + ($_ | Out-String)) | Set-Content -LiteralPath $resultPath -Encoding UTF8
    exit 1
}
