[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

function Find-RaceCompiler {
    $roots = @(
        (Join-Path $env:LOCALAPPDATA 'Microsoft\WinGet\Packages'),
        'C:\mingw64'
    ) | Where-Object { $_ -and (Test-Path -LiteralPath $_) }

    $candidates = foreach ($root in $roots) {
        Get-ChildItem -LiteralPath $root -Recurse -Filter gcc.exe -File -ErrorAction SilentlyContinue
    }

    foreach ($candidate in $candidates) {
        $version = (& $candidate.FullName --version 2>$null | Select-Object -First 1)
        if ($version -match 'ucrt-posix-seh|ucrt.*seh') {
            return $candidate
        }
    }

    throw 'No MinGW-w64 UCRT/SEH GCC found. Install the WinLibs POSIX/UCRT toolchain.'
}

$gcc = Find-RaceCompiler
$bin = Split-Path -Parent $gcc.FullName
$gxx = Join-Path $bin 'g++.exe'
if (-not (Test-Path -LiteralPath $gxx)) {
    throw "Matching g++ was not found: $gxx"
}

$env:CC = $gcc.FullName
$env:CXX = $gxx
$env:CGO_ENABLED = '1'
$env:PATH = "$bin;$env:PATH"

Write-Host "Using race compiler: $(& $gcc.FullName --version | Select-Object -First 1)"
Write-Host "Running: go test -race ./..."
& go test -race ./...
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
