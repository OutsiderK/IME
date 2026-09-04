#Requires -Version 5.1
#Requires -RunAsAdministrator

[CmdletBinding()]
param(
    [string] $StatusPath = (Join-Path $env:TEMP 'moqi-backend-f8safe2-status.json')
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$installRoot = Join-Path ${env:ProgramFiles(x86)} 'MoqiIM'
$source = Join-Path $repoRoot 'build\artifacts\backend\server-f8safe2.exe'
$target = Join-Path $installRoot 'moqi-ime\server.exe'
$launcher = Join-Path $installRoot 'MoqiLauncher.exe'
$aiStartupScript = Join-Path $installRoot 'moqi-ime\local-ai\ensure-local-ai.ps1'
$expectedHash = 'FDE486AF88A11D2F804AEB9E5C0AA04DC0F6F2354604615B4B01B4FBDCF55E2E'
$result = [ordered]@{
    version = 'f8safe2'
    success = $false
    backend = $target
    completedAt = $null
    error = $null
}

try {
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
        throw "Backend artifact was not found: $source"
    }
    $sourceHash = (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash
    if ($sourceHash -ne $expectedHash) {
        throw "Backend artifact hash mismatch: $sourceHash"
    }

    foreach ($process in Get-Process -Name 'MoqiLauncher', 'server' -ErrorAction SilentlyContinue) {
        try {
            if ($process.Path -in @($launcher, $target)) {
                Stop-Process -Id $process.Id -Force
            }
        }
        catch {
            # A process can exit between enumeration and inspection.
        }
    }
    Start-Sleep -Milliseconds 500

    $installedHash = ''
    if (Test-Path -LiteralPath $target -PathType Leaf) {
        $installedHash = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash
    }
    if ($installedHash -ne $sourceHash) {
        Copy-Item -LiteralPath $source -Destination $target -Force
    }

    $runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
    New-Item -Path $runKey -Force | Out-Null
    Set-ItemProperty -LiteralPath $runKey -Name 'Moqi Launcher' -Value ('"' + $launcher + '"')
    if (Test-Path -LiteralPath $aiStartupScript -PathType Leaf) {
        $aiCommand = 'powershell.exe -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "' + $aiStartupScript + '"'
        Set-ItemProperty -LiteralPath $runKey -Name 'Moqi Local AI' -Value $aiCommand
    }

    $desktopShell = New-Object -ComObject 'Shell.Application'
    $desktopShell.ShellExecute($launcher, '', $installRoot, 'open', 0)
    if (Test-Path -LiteralPath $aiStartupScript -PathType Leaf) {
        $aiArguments = '-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "' +
            $aiStartupScript + '"'
        $desktopShell.ShellExecute('powershell.exe', $aiArguments, $installRoot, 'open', 0)
    }

    $result.success = $true
    $result.completedAt = [DateTime]::UtcNow.ToString('o')
}
catch {
    $result.error = $_.Exception.ToString()
    $result.completedAt = [DateTime]::UtcNow.ToString('o')
    throw
}
finally {
    $result | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $StatusPath -Encoding UTF8
}
