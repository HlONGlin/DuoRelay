param([string]$KernelDirectory = (Join-Path $PSScriptRoot '../kernels'))
$ErrorActionPreference = 'Stop'

# Read the pinned Naive dependency instead of downloading a moving release.
$lockPath = Join-Path $PSScriptRoot '../kernels.lock'
$lock = [System.IO.File]::ReadAllText($lockPath)
$section = [regex]::Match($lock, '(?ms)^cronet:\s*\r?\n(?<body>.*)\z').Groups['body'].Value
$url = [regex]::Match($section, 'url: "(?<value>[^"]+)"').Groups['value'].Value
$digest = [regex]::Match($section, 'sha256: "(?<value>[a-f0-9]{64})"').Groups['value'].Value
if (!$url.StartsWith('https://raw.githubusercontent.com/SagerNet/cronet-go/') -or !$digest) {
    throw 'kernels.lock does not contain the pinned Windows amd64 Cronet dependency.'
}
New-Item -ItemType Directory -Force -Path $KernelDirectory | Out-Null
$destination = Join-Path $KernelDirectory 'libcronet.dll'
if (Test-Path -LiteralPath $destination) {
    if ((Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash.ToLowerInvariant() -ne $digest) {
        throw 'Existing libcronet.dll differs from kernels.lock; it was left untouched.'
    }
    Write-Output 'Pinned Cronet DLL verified.'
    return
}
$download = Join-Path $KernelDirectory ([guid]::NewGuid().ToString() + '.partial')
try {
    Invoke-WebRequest -Uri $url -OutFile $download -UseBasicParsing
    if ((Get-FileHash -LiteralPath $download -Algorithm SHA256).Hash.ToLowerInvariant() -ne $digest) {
        throw 'Cronet SHA256 verification failed.'
    }
    Move-Item -LiteralPath $download -Destination $destination
    Write-Output 'Pinned Cronet DLL installed and verified.'
} finally {
    if (Test-Path -LiteralPath $download) { Remove-Item -LiteralPath $download }
}
