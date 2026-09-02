param(
    [string]$Source = (Join-Path $PSScriptRoot 'build\moqi-ime\server.exe'),
    [string]$Destination = 'C:\Program Files (x86)\MoqiIM\moqi-ime\server.exe',
    [string]$Launcher = 'C:\Program Files (x86)\MoqiIM\MoqiLauncher.exe',
    [string]$ConfigSource = (Join-Path $PSScriptRoot '..\input_methods\rime\ai_config.json'),
    [string]$ConfigDestination = (Join-Path $env:APPDATA 'Moqi\ai_config.json')
)

$ErrorActionPreference = 'Stop'

$sourcePath = (Resolve-Path -LiteralPath $Source).Path
if (-not (Test-Path -LiteralPath $Destination)) {
    throw "Installed backend not found: $Destination"
}
if (-not (Test-Path -LiteralPath $Launcher)) {
    throw "Moqi launcher not found: $Launcher"
}

Start-Process -FilePath $Launcher -ArgumentList '/quit' -WindowStyle Hidden | Out-Null
$deadline = (Get-Date).AddSeconds(8)
do {
    Start-Sleep -Milliseconds 250
    $launcherProcesses = @(Get-Process -Name 'MoqiLauncher' -ErrorAction SilentlyContinue)
} while ($launcherProcesses.Count -gt 0 -and (Get-Date) -lt $deadline)

if ($launcherProcesses.Count -gt 0) {
    $launcherProcesses | Where-Object { $_.Path -eq $Launcher } | Stop-Process -Force
}
Get-Process -Name 'server' -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -eq $Destination } |
    Stop-Process -Force

Start-Sleep -Milliseconds 500
Copy-Item -LiteralPath $sourcePath -Destination $Destination -Force
if (Test-Path -LiteralPath $ConfigSource) {
    $configDirectory = Split-Path -Parent $ConfigDestination
    New-Item -ItemType Directory -Force -Path $configDirectory | Out-Null
    Copy-Item -LiteralPath $ConfigSource -Destination $ConfigDestination -Force
}
