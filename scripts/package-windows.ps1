[CmdletBinding()]
param(
    [string]$Version = "dev",
    [string]$OutputDirectory = "dist"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$staging = Join-Path $root (Join-Path $OutputDirectory "duorelay-windows-amd64-$Version")
$archive = Join-Path $root (Join-Path $OutputDirectory "duorelay-windows-amd64-$Version.zip")

Push-Location $root
try {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "Go is required" }
    if (-not (Get-Command npm -ErrorAction SilentlyContinue)) { throw "Node.js/npm is required" }

    $commit = (git rev-parse --short HEAD 2>$null)
    if (-not $commit) { $commit = "unknown" }
    $buildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

    if (Test-Path $staging) { Remove-Item -LiteralPath $staging -Recurse -Force }
    if (Test-Path $archive) { Remove-Item -LiteralPath $archive -Force }
    New-Item -ItemType Directory -Path $staging | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $staging "kernels") | Out-Null

    Push-Location (Join-Path $root "web")
    npm run build
    Pop-Location

    $binary = Join-Path $staging "duorelay.exe"
    $ldflags = "-X main.Version=$Version -X main.Commit=$commit -X main.BuildTime=$buildTime"
    go build -trimpath -ldflags $ldflags -o $binary ./cmd/duorelay

    foreach ($name in @("sing-box.exe", "mihomo.exe", "libcronet.dll")) {
        $source = Join-Path $root (Join-Path "kernels" $name)
        if (-not (Test-Path $source)) { throw "Missing kernel dependency: $source" }
        Copy-Item -LiteralPath $source -Destination (Join-Path $staging (Join-Path "kernels" $name))
    }
    foreach ($name in @("README.md", "LICENSE", "kernels.lock")) {
        $source = Join-Path $root $name
        if (Test-Path $source) { Copy-Item -LiteralPath $source -Destination (Join-Path $staging $name) }
    }
    @{
        version = $Version
        commit = $commit
        build_time = $buildTime
        platform = "windows/amd64"
        generated_at = (Get-Date).ToUniversalTime().ToString("o")
    } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $staging "build-info.json") -Encoding utf8

    Compress-Archive -Path (Join-Path $staging "*") -DestinationPath $archive -CompressionLevel Optimal
    Write-Host "Created $archive"
}
finally {
    Pop-Location
}
