[CmdletBinding()]
param(
    [int]$ApiPort = 18081,
    [int]$SingBoxPort = 18080,
    [int]$MihomoPort = 18082
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
    $settings.system_proxy = $true
    $null = Invoke-Api "PUT" "/api/config" $settings
    $results.Add("Windows system proxy settings save: PASS")

    $directSingBox = [ordered]@{
        log = [ordered]@{ level = "info" }
        inbounds = @([ordered]@{ type = "mixed"; tag = "mixed-in"; listen = "127.0.0.1"; listen_port = $SingBoxPort })
        outbounds = @([ordered]@{ type = "direct"; tag = "DIRECT" })
        route = [ordered]@{ final = "DIRECT" }
    }
    $applyRequest = [ordered]@{ kernel = "sing-box"; config = ($directSingBox | ConvertTo-Json -Depth 20 -Compress) }
    $null = Invoke-Api "POST" "/api/config/apply" $applyRequest
    $status = (Invoke-Api "GET" "/api/kernel/status" $null).data
    if (-not $status.running) { throw "sing-box was not reported running" }
    $proxyNow = Read-ProxySnapshot
    Assert-Equal $proxyNow.Enable 1 "system proxy enable state"
    Assert-Equal $proxyNow.Server ("127.0.0.1:" + $SingBoxPort) "system proxy target"
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
    $directMihomo = @"
mixed-port: $MihomoPort
allow-lan: false
mode: rule
log-level: info
proxies:
  - name: ACCEPT
    type: direct
proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - ACCEPT
rules:
  - MATCH,PROXY
"@
    $null = Invoke-Api "POST" "/api/config/apply" ([ordered]@{ kernel = "mihomo"; config = $directMihomo })
    $status = (Invoke-Api "GET" "/api/kernel/status" $null).data
    if (-not $status.running) { throw "mihomo was not reported running" }
    $results.Add("mihomo real start: PASS")
    $null = Invoke-Api "POST" "/api/kernel/stop" $null

    $tunConfig = [ordered]@{
        log = [ordered]@{ level = "info" }
        inbounds = @(
            [ordered]@{ type = "tun"; tag = "tun-in"; address = @("172.19.0.1/30"); auto_route = $true; stack = "system" },
            [ordered]@{ type = "mixed"; tag = "mixed-in"; listen = "127.0.0.1"; listen_port = $SingBoxPort }
        )
        outbounds = @([ordered]@{ type = "direct"; tag = "DIRECT" })
        route = [ordered]@{ auto_detect_interface = $true; final = "DIRECT" }
    }
    try {
        $null = Invoke-Api "POST" "/api/config/apply" ([ordered]@{ kernel = "sing-box"; config = ($tunConfig | ConvertTo-Json -Depth 20 -Compress) })
        $status = (Invoke-Api "GET" "/api/kernel/status" $null).data
        if (-not $status.running) { throw "TUN config was accepted but kernel is not running" }
        $results.Add("real TUN start: PASS")
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
$results | ForEach-Object { Write-Output $_ }
Write-Output ("Temporary acceptance data: " + $testRoot)
Write-Output "Final system proxy restoration: PASS"
