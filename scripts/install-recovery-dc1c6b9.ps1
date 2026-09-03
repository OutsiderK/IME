#Requires -Version 5.1
#Requires -RunAsAdministrator

[CmdletBinding()]
param(
    [string] $StatusPath = (Join-Path $env:TEMP 'moqi-recovery-dc1c6b9-status.json')
)

$ErrorActionPreference = 'Stop'
$recoveryVersion = 'dc1c6b9'
$repoRoot = Split-Path -Parent $PSScriptRoot
$installRoot = Join-Path ${env:ProgramFiles(x86)} 'MoqiIM'
$backendSource = Join-Path $repoRoot 'build\artifacts\backend\server-recovery.exe'
$frontend64Source = Join-Path $repoRoot 'build\artifacts\recovery\x64\MoqiTextService.dll'
$frontend32Source = Join-Path $repoRoot 'build\artifacts\recovery\win32\MoqiTextService.dll'
$backendTarget = Join-Path $installRoot 'moqi-ime\server.exe'
$frontend64Target = Join-Path $env:WINDIR "System32\MoqiTextService.$recoveryVersion.dll"
$frontend32Target = Join-Path $env:WINDIR "SysWOW64\MoqiTextService.$recoveryVersion.dll"
$launcherTarget = Join-Path $installRoot 'MoqiLauncher.exe'
$aiStartupScript = Join-Path $installRoot 'moqi-ime\local-ai\ensure-local-ai.ps1'
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$clsidKey = 'HKCU\Software\Classes\CLSID\{8F204C91-2D7A-4B3E-9E1F-6A5C0D8B2E7F}'

$expectedHashes = @{
    $backendSource  = 'C0B4CC8D7E0AD51D0819BCF9475BF7D47B8B8089BB6094B6C807BB815BD1D877'
    $frontend64Source = 'FE6F92E172085E35A26C804CE23A70C9C302DDE8605B5976ED67B5BE488CF01A'
    $frontend32Source = 'B14043DB24367A26E711E5D75C1926F2F054A8EF41A4331343E2D267A641D146'
}

function Assert-RecoveryArtifact {
    param([Parameter(Mandatory = $true)][string] $Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Recovery artifact was not found: $Path"
    }
    $actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash
    if ($actual -ne $expectedHashes[$Path]) {
        throw "Recovery artifact hash mismatch: $Path (actual $actual)"
    }
}

function Stop-InstalledMoqiRuntime {
    $allowedPaths = @($launcherTarget, $backendTarget)
    foreach ($process in Get-Process -Name 'MoqiLauncher', 'server' -ErrorAction SilentlyContinue) {
        try {
            if ($allowedPaths -contains $process.Path) {
                Stop-Process -Id $process.Id -Force
            }
        }
        catch {
            # A process can exit between enumeration and inspection.
        }
    }
}

$result = [ordered]@{
    version = $recoveryVersion
    success = $false
    backend = $backendTarget
    frontend64 = $frontend64Target
    frontend32 = $frontend32Target
    completedAt = $null
    error = $null
}

try {
    Assert-RecoveryArtifact -Path $backendSource
    Assert-RecoveryArtifact -Path $frontend64Source
    Assert-RecoveryArtifact -Path $frontend32Source

    Stop-InstalledMoqiRuntime
    Start-Sleep -Milliseconds 500

    Copy-Item -LiteralPath $backendSource -Destination $backendTarget -Force
    Copy-Item -LiteralPath $frontend64Source -Destination $frontend64Target -Force
    Copy-Item -LiteralPath $frontend32Source -Destination $frontend32Target -Force

    $regsvr64 = Join-Path $env:WINDIR 'System32\regsvr32.exe'
    $regsvr32 = Join-Path $env:WINDIR 'SysWOW64\regsvr32.exe'
    $register64 = Start-Process -FilePath $regsvr64 -ArgumentList @(
        '/s', ('"' + $frontend64Target + '"')
    ) -Wait -PassThru -WindowStyle Hidden
    if ($register64.ExitCode -ne 0) {
        throw "64-bit regsvr32 failed with exit code $($register64.ExitCode)"
    }
    $register32 = Start-Process -FilePath $regsvr32 -ArgumentList @(
        '/s', ('"' + $frontend32Target + '"')
    ) -Wait -PassThru -WindowStyle Hidden
    if ($register32.ExitCode -ne 0) {
        throw "32-bit regsvr32 failed with exit code $($register32.ExitCode)"
    }

    # Remove temporary per-user COM overrides from earlier recovery attempts.
    & reg.exe delete $clsidKey /f /reg:64 2>$null | Out-Null
    & reg.exe delete $clsidKey /f /reg:32 2>$null | Out-Null

    New-Item -Path $runKey -Force | Out-Null
    Set-ItemProperty -LiteralPath $runKey -Name 'Moqi Launcher' -Value ('"' + $launcherTarget + '"')
    if (Test-Path -LiteralPath $aiStartupScript -PathType Leaf) {
        $aiCommand = 'powershell.exe -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "' + $aiStartupScript + '"'
        Set-ItemProperty -LiteralPath $runKey -Name 'Moqi Local AI' -Value $aiCommand
    }

    Start-Process -FilePath $launcherTarget -WorkingDirectory $installRoot -WindowStyle Hidden
    if (Test-Path -LiteralPath $aiStartupScript -PathType Leaf) {
        Start-Process -FilePath 'powershell.exe' -ArgumentList @(
            '-NoProfile', '-WindowStyle', 'Hidden', '-ExecutionPolicy', 'Bypass',
            '-File', ('"' + $aiStartupScript + '"')
        ) -WindowStyle Hidden
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
