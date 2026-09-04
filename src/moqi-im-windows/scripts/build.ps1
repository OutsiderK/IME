#Requires -Version 5.1
<#
.SYNOPSIS
  Build Win32 and x64 Moqi IM for Windows binaries with CMake.

.PARAMETER RepoRoot
  Root of moqi-im-windows (defaults to the parent directory of this script).

.PARAMETER Win32BuildDir
  CMake Win32 build directory (default: RepoRoot\build-vs32).

.PARAMETER X64BuildDir
  CMake x64 build directory (default: RepoRoot\build-vs64).

.PARAMETER Configuration
  Build configuration (default: Release).

.PARAMETER Generator
  CMake generator (default: Visual Studio 17 2022).

.PARAMETER ProtobufRoot
  Optional local protobuf/protoc install root passed to CMake as MOQI_PROTOBUF_ROOT.

.PARAMETER ProtobufSourceDir
  Optional local protobuf source tree passed to CMake as MOQI_PROTOBUF_SOURCE_DIR.

.PARAMETER Clean
  Remove both build directories before configuring. Use this for release packages
  so binaries cannot contain stale object files from an earlier header layout.
#>
param(
  [string] $RepoRoot = "",
  [string] $Win32BuildDir = "",
  [string] $X64BuildDir = "",
  [string] $Configuration = "Release",
  [string] $Generator = "Visual Studio 17 2022",
  [string] $ProtobufRoot = "",
  [string] $ProtobufSourceDir = "",
  [switch] $Clean
)

$ErrorActionPreference = "Stop"

function Invoke-Step {
  param(
    [string] $FilePath,
    [string[]] $ArgumentList
  )

  Write-Host ">> $FilePath $($ArgumentList -join ' ')"
  & $FilePath @ArgumentList
  if ($LASTEXITCODE -ne 0) {
    throw "Command failed with exit code ${LASTEXITCODE}: $FilePath"
  }
}

function Invoke-CMakeConfigure {
  param(
    [string[]] $ArgumentList,
    [string] $BuildDir
  )

  # On affected Windows hosts, probing C and C++ against the same cache can
  # crash CMake. Probe C in the real build tree and C++ in a temporary sibling,
  # then reuse the generated compiler description for the full configuration.
  Invoke-Step -FilePath "cmake" -ArgumentList ($ArgumentList + "-DMOQI_BOOTSTRAP_LANGUAGE=C")

  $cxxBuildDir = "${BuildDir}-compiler-cxx"
  if (Test-Path -LiteralPath $cxxBuildDir) {
    Remove-Item -LiteralPath $cxxBuildDir -Recurse -Force
  }
  $cxxArguments = [System.Collections.Generic.List[string]]::new()
  for ($index = 0; $index -lt $ArgumentList.Count; $index++) {
    if ($ArgumentList[$index] -eq "-B") {
      $cxxArguments.Add("-B")
      $cxxArguments.Add($cxxBuildDir)
      $index++
    }
    else {
      $cxxArguments.Add($ArgumentList[$index])
    }
  }
  Invoke-Step -FilePath "cmake" -ArgumentList ($cxxArguments.ToArray() + "-DMOQI_BOOTSTRAP_LANGUAGE=CXX")

  $cxxCompilerFile = Get-ChildItem -LiteralPath (Join-Path $cxxBuildDir "CMakeFiles") -Filter "CMakeCXXCompiler.cmake" -File -Recurse |
    Select-Object -First 1
  $cCompilerFile = Get-ChildItem -LiteralPath (Join-Path $BuildDir "CMakeFiles") -Filter "CMakeCCompiler.cmake" -File -Recurse |
    Select-Object -First 1
  if (-not $cxxCompilerFile -or -not $cCompilerFile) {
    throw "CMake compiler bootstrap did not produce the expected compiler descriptions."
  }
  $platformInfoDir = Split-Path -Parent $cCompilerFile.FullName
  Copy-Item -LiteralPath $cxxCompilerFile.FullName -Destination $platformInfoDir -Force
  $cxxAbiFile = Join-Path $cxxCompilerFile.DirectoryName "CMakeDetermineCompilerABI_CXX.bin"
  if (Test-Path -LiteralPath $cxxAbiFile) {
    Copy-Item -LiteralPath $cxxAbiFile -Destination $platformInfoDir -Force
  }

  Invoke-Step -FilePath "cmake" -ArgumentList ($ArgumentList + "-DMOQI_BOOTSTRAP_LANGUAGE=")
}

function Resolve-ProtobufSourceDir {
  param(
    [string] $RequestedPath,
    [string] $RepoRoot
  )

  $candidates = @()
  if (-not [string]::IsNullOrWhiteSpace($RequestedPath)) {
    $candidates += $RequestedPath
  }
  if (-not [string]::IsNullOrWhiteSpace($env:MOQI_PROTOBUF_SOURCE_DIR)) {
    $candidates += $env:MOQI_PROTOBUF_SOURCE_DIR
  }
  $candidates += (Join-Path $RepoRoot "third_party\protobuf-33.5")
  if (-not [string]::IsNullOrWhiteSpace($env:USERPROFILE)) {
    $cacheRoot = Join-Path $env:USERPROFILE ".cache\moqi-protobuf"
    $candidates += @(
      (Join-Path $cacheRoot "protobuf-33.5"),
      (Join-Path $cacheRoot "protobuf-34.1"),
      (Join-Path $cacheRoot "protobuf-29.5")
    )
  }

  foreach ($candidate in $candidates) {
    if ([string]::IsNullOrWhiteSpace($candidate)) {
      continue
    }
    $fullPath = [System.IO.Path]::GetFullPath($candidate)
    if (Test-Path -LiteralPath (Join-Path $fullPath "CMakeLists.txt")) {
      return $fullPath
    }
  }

  return ""
}

function Resolve-ProtobufRoot {
  param(
    [string] $RequestedPath,
    [string] $RepoRoot
  )

  $candidates = @()
  if (-not [string]::IsNullOrWhiteSpace($RequestedPath)) {
    $candidates += $RequestedPath
  }
  if (-not [string]::IsNullOrWhiteSpace($env:MOQI_PROTOBUF_ROOT)) {
    $candidates += $env:MOQI_PROTOBUF_ROOT
  }
  $candidates += (Join-Path $RepoRoot "third_party\protoc-33.5-win64")
  $defaultRoot = "D:\a_dev\protoc-33.5-win64"
  if (Test-Path -LiteralPath $defaultRoot) {
    $candidates += $defaultRoot
  }

  foreach ($candidate in $candidates) {
    if ([string]::IsNullOrWhiteSpace($candidate)) {
      continue
    }
    $fullPath = [System.IO.Path]::GetFullPath($candidate)
    if ((Test-Path -LiteralPath (Join-Path $fullPath "bin\protoc.exe")) -or
        (Test-Path -LiteralPath (Join-Path $fullPath "include"))) {
      return $fullPath
    }
  }

  return ""
}

$scriptRepoRoot = Join-Path $PSScriptRoot ".."
if (-not $RepoRoot) { $RepoRoot = $scriptRepoRoot }
$RepoRoot = [System.IO.Path]::GetFullPath($RepoRoot)

if (-not $Win32BuildDir) { $Win32BuildDir = Join-Path $RepoRoot "build-vs32" }
if (-not $X64BuildDir) { $X64BuildDir = Join-Path $RepoRoot "build-vs64" }
$Win32BuildDir = [System.IO.Path]::GetFullPath($Win32BuildDir)
$X64BuildDir = [System.IO.Path]::GetFullPath($X64BuildDir)
$ProtobufRoot = Resolve-ProtobufRoot -RequestedPath $ProtobufRoot -RepoRoot $RepoRoot
$ProtobufSourceDir = Resolve-ProtobufSourceDir -RequestedPath $ProtobufSourceDir -RepoRoot $RepoRoot

if ($Clean) {
  foreach ($buildDir in @($Win32BuildDir, $X64BuildDir)) {
    $pathRoot = [System.IO.Path]::GetPathRoot($buildDir)
    if ($buildDir -eq $pathRoot -or $buildDir -eq $RepoRoot) {
      throw "Refusing to clean unsafe build directory: $buildDir"
    }
    if (Test-Path -LiteralPath $buildDir) {
      Write-Host "[CLEAN] Removing build directory: $buildDir"
      Remove-Item -LiteralPath $buildDir -Recurse -Force
    }
  }
}

# Ninja parses MSVC /showIncludes output by a localized text prefix. Keeping the
# compiler diagnostics in English prevents non-ASCII code-page conversion from
# silently dropping header dependencies and reusing ABI-incompatible objects.
$previousVsLang = $env:VSLANG
$env:VSLANG = "1033"

$commonConfigureArgs = @("-S", $RepoRoot)
if (-not [string]::IsNullOrWhiteSpace($ProtobufRoot)) {
  Write-Host "[INFO] Using local protobuf root: $ProtobufRoot"
  $commonConfigureArgs += "-DMOQI_PROTOBUF_ROOT=$ProtobufRoot"
  $protocExe = Join-Path $ProtobufRoot "bin\protoc.exe"
  if (Test-Path -LiteralPath $protocExe) {
    $commonConfigureArgs += "-DMOQI_PROTOC_EXECUTABLE=$protocExe"
  }
}
if (-not [string]::IsNullOrWhiteSpace($ProtobufSourceDir)) {
  Write-Host "[INFO] Using local protobuf source: $ProtobufSourceDir"
  $commonConfigureArgs += "-DMOQI_PROTOBUF_SOURCE_DIR=$ProtobufSourceDir"
}
$win32ConfigureArgs = $commonConfigureArgs + @(
  "-B", $Win32BuildDir,
  "-G", $Generator,
  "-A", "Win32"
)
$x64ConfigureArgs = $commonConfigureArgs + @(
  "-B", $X64BuildDir,
  "-G", $Generator,
  "-A", "x64"
)

try {
  Invoke-CMakeConfigure -ArgumentList $win32ConfigureArgs -BuildDir $Win32BuildDir
  Invoke-Step -FilePath "cmake" -ArgumentList @(
    "--build", $Win32BuildDir,
    "--config", $Configuration,
    "--target", "MoqiTextService", "MoqiLauncher", "SetupHelper"
  )

  Invoke-CMakeConfigure -ArgumentList $x64ConfigureArgs -BuildDir $X64BuildDir
  Invoke-Step -FilePath "cmake" -ArgumentList @(
    "--build", $X64BuildDir,
    "--config", $Configuration,
    "--target", "MoqiTextService"
  )
}
finally {
  $env:VSLANG = $previousVsLang
}

Write-Host "OK: Win32 $Configuration (runtime targets), x64 $Configuration (MoqiTextService)."
