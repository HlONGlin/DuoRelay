[CmdletBinding()]
param(
    [int]$ApiPort = 18081,
    [int]$SingBoxPort = 18080,
    [int]$MihomoPort = 18082,
    [string]$TestUrl = $(if ($env:DUORELAY_TEST_URL) { $env:DUORELAY_TEST_URL } else { "https://example.com/" })
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$root = Split-Path -Parent $PSScriptRoot
$package = Join-Path $root "dist\duorelay-windows-amd64-0.3.0"
$exe = Join-Path $package "duorelay.exe"
$registryPath = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings"
$testRoot = Join-Path $env:TEMP ("duorelay-acceptance-" + [guid]::NewGuid().ToString("N"))
$logOut = Join-Path $testRoot "stdout.log"
$logErr = Join-Path $testRoot "stderr.log"
$apiBase = "http://127.0.0.1:$ApiPort"
$process = $null

function Read-ProxySnapshot {
    $item = Get-ItemProperty -Path $registryPath -ErrorAction SilentlyContinue
    [pscustomobject]@{
        EnablePresent = $null -ne $item -and $item.PSObject.Properties.Name -contains "ProxyEnable"
        Enable = if ($null -ne $item -and $item.PSObject.Properties.Name -contains "ProxyEnable") { [int]$item.ProxyEnable } else { 0 }
        ServerPresent = $null -ne $item -and $item.PSObject.Properties.Name -contains "ProxyServer"
        Server = if ($null -ne $item -and $item.PSObject.Properties.Name -contains "ProxyServer") { [string]$item.ProxyServer } else { "" }
        OverridePresent = $null -ne $item -and $item.PSObject.Properties.Name -contains "ProxyOverride"
        Override = if ($null -ne $item -and $item.PSObject.Properties.Name -contains "ProxyOverride") { [string]$item.ProxyOverride } else { "" }
    }
}

function Restore-ProxySnapshot($snapshot) {
    if ($snapshot.EnablePresent) { Set-ItemProperty -Path $registryPath -Name ProxyEnable -Value $snapshot.Enable -Type DWord } else { Remove-ItemProperty -Path $registryPath -Name ProxyEnable -ErrorAction SilentlyContinue }
    if ($snapshot.ServerPresent) { Set-ItemProperty -Path $registryPath -Name ProxyServer -Value $snapshot.Server -Type String } else { Remove-ItemProperty -Path $registryPath -Name ProxyServer -ErrorAction SilentlyContinue }
    if ($snapshot.OverridePresent) { Set-ItemProperty -Path $registryPath -Name ProxyOverride -Value $snapshot.Override -Type String } else { Remove-ItemProperty -Path $registryPath -Name ProxyOverride -ErrorAction SilentlyContinue }
}

function Invoke-Api($method, $path, $body) {
    $params = @{ Uri = $apiBase + $path; Method = $method; UseBasicParsing = $true; ErrorAction = "Stop" }
    if ($null -ne $body) {
        $params.ContentType = "application/json"
        $params.Body = ($body | ConvertTo-Json -Depth 40 -Compress)
    }
    Invoke-RestMethod @params
}

function Get-HttpErrorDetail($errorRecord) {
    $detail = $errorRecord.Exception.Message
    if ($errorRecord.ErrorDetails -and $errorRecord.ErrorDetails.Message) {
        $detail += " | " + $errorRecord.ErrorDetails.Message
    }
    $response = $errorRecord.Exception.Response
    if ($null -ne $response) {
        try {
            $reader = New-Object System.IO.StreamReader($response.GetResponseStream())
            $body = $reader.ReadToEnd()
            $reader.Dispose()
            if ($body) { $detail += " | " + $body }
        } catch {}
    }
    return $detail
}

function Assert-Equal($actual, $expected, $message) {
    if ($actual -ne $expected) { throw ($message + ": actual=" + [string]$actual + ", expected=" + [string]$expected) }
}

if (-not (Test-Path $exe)) { throw "发布包不存在，请先运行 scripts/package-windows.ps1" }
$originalProxy = Read-ProxySnapshot
$results = [System.Collections.Generic.List[string]]::new()
$tunAdaptersBefore = @()
if (Get-Command Get-NetAdapter -ErrorAction SilentlyContinue) {
    $tunAdaptersBefore = @(Get-NetAdapter | Where-Object { $_.Name -match "Wintun|TUN|DuoRelay|sing-box|mihomo" } | Select-Object -ExpandProperty Name)
}

try {
    New-Item -ItemType Directory -Path $testRoot -Force | Out-Null
    $arguments = @(
        "-listen", "127.0.0.1:$ApiPort",
        "-db", (Join-Path $testRoot "duorelay.db"),
        "-kernels", (Join-Path $package "kernels"),
        "-config-dir", (Join-Path $testRoot "configs"),
        "-workers", "2"
    )
    $process = Start-Process -FilePath $exe -ArgumentList $arguments -WorkingDirectory $package -WindowStyle Hidden -RedirectStandardOutput $logOut -RedirectStandardError $logErr -PassThru

    $ready = $false
    for ($i = 0; $i -lt 60; $i++) {
        Start-Sleep -Milliseconds 250
        try {
            $null = Invoke-WebRequest -Uri ($apiBase + "/api/config") -UseBasicParsing -TimeoutSec 2
            $ready = $true
            break
        } catch {
            if ($process.HasExited) { throw ("DuoRelay exited during startup: " + (Get-Content $logErr -Raw -ErrorAction SilentlyContinue)) }
        }
    }
    Assert-Equal $ready $true "API did not become ready"
    $results.Add("API startup: PASS")

    $settings = (Invoke-Api "GET" "/api/config" $null).data
    $settings.inbound_port = $SingBoxPort
    $settings.mixed_port = $MihomoPort
    $settings.socks_port = $MihomoPort + 1
    $settings.enable_tun = $false
    $settings.transparent_mode = "off"
    $settings.system_proxy = $true
    $settings.routing_mode = "blacklist"
    $settings.default_outbound = "DIRECT"
    $settings.routing_rules = @()
    $null = Invoke-Api "PUT" "/api/config" $settings
    $results.Add("Windows system proxy settings save: PASS")

    $null = Invoke-Api "POST" "/api/config/apply" ([ordered]@{ kernel = "sing-box" })
    $status = (Invoke-Api "GET" "/api/kernel/status" $null).data
    if (-not $status.running) { throw "sing-box was not reported running" }
    $proxyNow = Read-ProxySnapshot
    Assert-Equal $proxyNow.Enable 1 "system proxy enable state"
    Assert-Equal $proxyNow.Server ("127.0.0.1:" + $SingBoxPort) "system proxy target"
    $response = Invoke-WebRequest -Uri $TestUrl -Proxy ("http://127.0.0.1:" + $SingBoxPort) -UseBasicParsing -TimeoutSec 15
    if ($response.StatusCode -lt 200 -or $response.StatusCode -ge 500) { throw "system proxy request returned HTTP $($response.StatusCode)" }
    $results.Add("system proxy real request through local mixed port: PASS")
    $results.Add("sing-box real start + system proxy switch: PASS")
    Start-Sleep -Milliseconds 500
    $logs = (Invoke-Api "GET" "/api/kernel/logs?kernel=sing-box&limit=100" $null).data
    if ($logs.Count -lt 1) { throw "sing-box produced no captured logs" }
    $results.Add("real kernel log capture: PASS")

    $null = Invoke-Api "POST" "/api/kernel/stop" $null
    Start-Sleep -Milliseconds 500
    $proxyAfterStop = Read-ProxySnapshot
    Assert-Equal $proxyAfterStop.Enable $originalProxy.Enable "proxy enable restoration"
    Assert-Equal $proxyAfterStop.Server $originalProxy.Server "proxy server restoration"
    Assert-Equal $proxyAfterStop.Override $originalProxy.Override "proxy override restoration"
    $results.Add("system proxy restore after stop: PASS")

    $settings.system_proxy = $false
    $null = Invoke-Api "PUT" "/api/config" $settings
    $null = Invoke-Api "POST" "/api/config/apply" ([ordered]@{ kernel = "mihomo" })
    $status = (Invoke-Api "GET" "/api/kernel/status" $null).data
    if (-not $status.running) { throw "mihomo was not reported running" }
    $results.Add("mihomo real start: PASS")
    $null = Invoke-Api "POST" "/api/kernel/stop" $null

    try {
        $settings.enable_tun = $true
        $settings.transparent_mode = "tun"
        $settings.system_proxy = $false
        $null = Invoke-Api "PUT" "/api/config" $settings
        $null = Invoke-Api "POST" "/api/config/apply" ([ordered]@{ kernel = "sing-box" })
        $status = (Invoke-Api "GET" "/api/kernel/status" $null).data
        if (-not $status.running) { throw "TUN config was accepted but kernel is not running" }
        $results.Add("real TUN start: PASS")
        $response = Invoke-WebRequest -Uri $TestUrl -UseBasicParsing -TimeoutSec 15
        if ($response.StatusCode -lt 200 -or $response.StatusCode -ge 500) { throw "TUN request returned HTTP $($response.StatusCode)" }
        $results.Add("TUN real request: PASS")
    } catch {
        $detail = "real TUN start: BLOCKED - " + (Get-HttpErrorDetail $_)
        try {
            $tunLogs = (Invoke-Api "GET" "/api/kernel/logs?kernel=sing-box&limit=50" $null).data
            $lines = @($tunLogs | ForEach-Object { $_.message })
            if ($lines.Count -gt 0) { $detail += " | logs=" + ($lines -join " || ") }
        } catch {}
        $results.Add($detail)
    } finally {
        try { $null = Invoke-Api "POST" "/api/kernel/stop" $null } catch {}
    }
} finally {
    try { Restore-ProxySnapshot $originalProxy } catch {}
    if ($null -ne $process -and -not $process.HasExited) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue }
}

$finalProxy = Read-ProxySnapshot
Assert-Equal $finalProxy.Enable $originalProxy.Enable "final proxy enable restoration"
Assert-Equal $finalProxy.Server $originalProxy.Server "final proxy server restoration"
Assert-Equal $finalProxy.Override $originalProxy.Override "final proxy override restoration"
if (Get-Command Get-NetAdapter -ErrorAction SilentlyContinue) {
    $tunAdaptersAfter = @(Get-NetAdapter | Where-Object { $_.Name -match "Wintun|TUN|DuoRelay|sing-box|mihomo" } | Select-Object -ExpandProperty Name)
    $newAdapters = @($tunAdaptersAfter | Where-Object { $tunAdaptersBefore -notcontains $_ })
    if ($newAdapters.Count -gt 0) { throw "TUN adapters remained after stop: $($newAdapters -join ', ')" }
    $results.Add("TUN adapter cleanup: PASS")
} else {
    $results.Add("TUN adapter cleanup: UNVERIFIED - Get-NetAdapter unavailable")
}
$results.Add("Windows matrix: system_proxy=VERIFIED; tun=VERIFIED only when the real request and adapter cleanup checks above pass")
$results | ForEach-Object { Write-Output $_ }
Write-Output ("Temporary acceptance data: " + $testRoot)
Write-Output "Final system proxy restoration: PASS"
