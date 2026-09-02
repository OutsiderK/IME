$ErrorActionPreference = 'Stop'

$stage = 'C:\Users\joeyK\AppData\Local\Temp\moqi-register-v2'
$dll64 = Join-Path $stage 'x64\MoqiTextService.dll'
$dll32 = Join-Path $stage 'x86\MoqiTextService.dll'
$resultPath = 'C:\Users\joeyK\AppData\Local\Temp\moqi-register-v2-result.log'

try {
    foreach ($path in @($dll64, $dll32)) {
        if (-not (Test-Path -LiteralPath $path)) {
            throw "Missing TSF DLL: $path"
        }
    }

    $register64 = Start-Process -FilePath 'C:\Windows\System32\regsvr32.exe' `
        -ArgumentList @('/s', $dll64) -Wait -PassThru -WindowStyle Hidden
    if ($register64.ExitCode -ne 0) {
        throw "64-bit regsvr32 failed: $($register64.ExitCode)"
    }

    $register32 = Start-Process -FilePath 'C:\Windows\SysWOW64\regsvr32.exe' `
        -ArgumentList @('/s', $dll32) -Wait -PassThru -WindowStyle Hidden
    if ($register32.ExitCode -ne 0) {
        throw "32-bit regsvr32 failed: $($register32.ExitCode)"
    }

    'OK' | Set-Content -LiteralPath $resultPath -Encoding UTF8
}
catch {
    ("FAILED`r`n" + ($_ | Out-String)) | Set-Content -LiteralPath $resultPath -Encoding UTF8
    exit 1
}
