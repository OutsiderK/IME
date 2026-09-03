#Requires -Version 5.1
#Requires -RunAsAdministrator

[CmdletBinding()]
param(
    [string] $StatusPath = (Join-Path $env:TEMP 'moqi-ghost-ui-v1-status.json')
)

$ErrorActionPreference = 'Stop'
$version = 'ghost-ui-v1'
$repoRoot = Split-Path -Parent $PSScriptRoot
$installRoot = Join-Path ${env:ProgramFiles(x86)} 'MoqiIM'
$backendSource = Join-Path $repoRoot 'build\artifacts\backend\server-ghost-ui-v1.exe'
$frontend64Source = Join-Path $repoRoot 'build\artifacts\ghost-ui-v1\x64\MoqiTextService.dll'
$frontend32Source = Join-Path $repoRoot 'build\artifacts\ghost-ui-v1\win32\MoqiTextService.dll'
$backendTarget = Join-Path $installRoot 'moqi-ime\server.exe'
$frontend64Target = Join-Path $env:WINDIR "System32\MoqiTextService.$version.dll"
$frontend32Target = Join-Path $env:WINDIR "SysWOW64\MoqiTextService.$version.dll"
$launcherTarget = Join-Path $installRoot 'MoqiLauncher.exe'
$aiStartupScript = Join-Path $installRoot 'moqi-ime\local-ai\ensure-local-ai.ps1'
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$clsidKey = 'HKCU\Software\Classes\CLSID\{8F204C91-2D7A-4B3E-9E1F-6A5C0D8B2E7F}'

$expectedHashes = @{
    $backendSource = '230E353C4DE778368DE919E787A9C051647032BF414CC599BAA4C4273CABBCCD'
    $frontend64Source = '77096303480C6BCBFFBF9F59AB893F716695B9F86E60A4FB3C179EE6970D8A6A'
    $frontend32Source = 'E1FA14ADD339133F002A66DA56A71B2F4659A0D5EDD1A240E58669DBC14F40ED'
}

function Assert-Artifact {
    param([Parameter(Mandatory = $true)][string] $Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Artifact was not found: $Path"
    }
    $actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash
    if ($actual -ne $expectedHashes[$Path]) {
        throw "Artifact hash mismatch: $Path (actual $actual)"
    }
}

function Install-Artifact {
    param(
        [Parameter(Mandatory = $true)][string] $Source,
        [Parameter(Mandatory = $true)][string] $Destination
    )

    if (Test-Path -LiteralPath $Destination -PathType Leaf) {
        $sourceHash = (Get-FileHash -LiteralPath $Source -Algorithm SHA256).Hash
        $destinationHash = (Get-FileHash -LiteralPath $Destination -Algorithm SHA256).Hash
        if ($sourceHash -eq $destinationHash) {
            return
        }
    }
    Copy-Item -LiteralPath $Source -Destination $Destination -Force
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
    version = $version
    success = $false
    backend = $backendTarget
    frontend64 = $frontend64Target
    frontend32 = $frontend32Target
    completedAt = $null
    error = $null
}

try {
    Assert-Artifact -Path $backendSource
    Assert-Artifact -Path $frontend64Source
    Assert-Artifact -Path $frontend32Source

    Stop-InstalledMoqiRuntime
    Start-Sleep -Milliseconds 500

    Install-Artifact -Source $backendSource -Destination $backendTarget
    Install-Artifact -Source $frontend64Source -Destination $frontend64Target
    Install-Artifact -Source $frontend32Source -Destination $frontend32Target

    $regExe = Join-Path $env:WINDIR 'System32\reg.exe'
    foreach ($registryView in @('/reg:64', '/reg:32')) {
        Start-Process -FilePath $regExe -ArgumentList @(
            'delete', $clsidKey, '/f', $registryView
        ) -Wait -WindowStyle Hidden
    }

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

    New-Item -Path $runKey -Force | Out-Null
    Set-ItemProperty -LiteralPath $runKey -Name 'Moqi Launcher' -Value ('"' + $launcherTarget + '"')
    if (Test-Path -LiteralPath $aiStartupScript -PathType Leaf) {
        $aiCommand = 'powershell.exe -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File "' + $aiStartupScript + '"'
        Set-ItemProperty -LiteralPath $runKey -Name 'Moqi Local AI' -Value $aiCommand
    }

    $desktopShell = New-Object -ComObject 'Shell.Application'
    $desktopShell.ShellExecute($launcherTarget, '', $installRoot, 'open', 0)
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
