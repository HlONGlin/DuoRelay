# Clash API Package

## Overview

The `clashapi` package provides a unified interface to interact with Clash API implementations from both sing-box and mihomo kernels. It abstracts away the differences between the two implementations while preserving kernel-specific features when needed.

## Architecture

```
clashapi/
├── types.go       - Common data structures
├── client.go      - HTTP client for Clash API
├── traffic.go     - WebSocket traffic sampling client
├── adapter.go     - Unified adapter layer
└── adapter_test.go - Integration tests
```

## Usage

### Creating an Adapter

```go
import "github.com/holi/duorelay/internal/clashapi"

// For sing-box
adapter := clashapi.NewAdapter(
    "http://127.0.0.1:19094",
    "your-secret",
    clashapi.KernelSingBox,
)
defer adapter.Close()

// For mihomo
adapter := clashapi.NewAdapter(
    "http://127.0.0.1:19093",
    "your-secret",
    clashapi.KernelMihomo,
)
defer adapter.Close()
```

### Getting Version Information

```go
ctx := context.Background()
version, err := adapter.GetVersion(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Version: %s\n", version.Version)
```

### Listing Proxies

```go
proxies, err := adapter.GetProxies(ctx)
if err != nil {
    log.Fatal(err)
}

for name, proxy := range proxies.Proxies {
    fmt.Printf("Proxy: %s (Type: %s, UDP: %v)\n", 
        name, proxy.Type, proxy.UDP)
}
```

### Getting Active Connections

```go
connections, err := adapter.GetConnections(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Active: %d, Download: %d, Upload: %d\n",
    len(connections.Connections),
    connections.DownloadTotal,
    connections.UploadTotal)
```

### Sampling Traffic

`ReadTraffic` opens the `/traffic` WebSocket, reads one `{up, down}` sample,
and closes it. The call honors context cancellation and sends the configured
Bearer token during the WebSocket handshake.

```go
traffic, err := adapter.ReadTraffic(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Up: %d B/s, Down: %d B/s\n", traffic.Up, traffic.Down)
```

### Testing Proxy Delay

```go
delay, err := adapter.TestProxyDelay(ctx, "proxy-name", "", 5000)
if err != nil {
    log.Printf("Delay test failed: %v", err)
} else {
    fmt.Printf("Delay: %d ms\n", delay.Delay)
}
```

## API Compatibility

| Endpoint | sing-box | mihomo | Notes |
|----------|----------|--------|-------|
| `/version` | ✅ | ✅ | Minor format differences |
| `/proxies` | ✅ | ✅ | mihomo returns more metadata |
| `/connections` | ✅ | ✅ | Both support, minor differences |
| `/proxies/{name}/delay` | ✅ | ✅ | Compatible |
| `/traffic` (WebSocket) | ✅ | ✅ | Supported through `ReadTraffic` |
| `/logs` (WebSocket) | ✅ | ✅ | Not yet implemented |

See [docs/clashapi-diff.md](../../docs/clashapi-diff.md) for detailed API differences.

## Testing

### Prerequisites

Start the respective kernel with test configuration:

```bash
# For sing-box
./kernels/sing-box.exe run -c testdata/configs/singbox-minimal.json

# For mihomo
./kernels/mihomo.exe -f testdata/configs/mihomo-minimal.yaml
```

### Run Tests

The default package tests use an in-process `httptest` Clash API mock and do not
require either kernel to be running. Live kernel integration tests are opt-in:

```bash
DUORELAY_CLASHAPI_INTEGRATION=1 go test -v ./internal/clashapi -run 'Test(SingBox|Mihomo)ClashAPI'
```

```bash
# Test sing-box API
DUORELAY_CLASHAPI_INTEGRATION=1 go test -v ./internal/clashapi -run TestSingBoxClashAPI

# Test mihomo API
DUORELAY_CLASHAPI_INTEGRATION=1 go test -v ./internal/clashapi -run TestMihomoClashAPI

# Run automated test script
bash scripts/test-clashapi.sh
```

## Kernel-Specific Features

### mihomo-only Fields

The following fields are only available when using mihomo:

- `Proxy.Alive` - Proxy availability status
- `Proxy.ID` - Unique identifier
- `Proxy.DialerProxy` - Chain proxy configuration
- `Proxy.Extra` - Additional metadata
- `ConnectionsResponse.Memory` - Memory usage

These fields will be zero-valued when using sing-box.

### Built-in Proxies

**sing-box:**
- `direct` - Direct connection
- `block` - Block traffic
- `GLOBAL` - Global proxy group

**mihomo:**
- `DIRECT` - Direct connection
- `REJECT` - Reject traffic
- `REJECT-DROP` - Drop packets silently
- `GLOBAL` - Global proxy group
- `PASS` - Pass through
- `PASS-RULE` - Pass through with rules
- `COMPATIBLE` - Compatibility mode

## Error Handling

The adapter returns standard Go errors. Network errors, timeouts, and API errors are wrapped with context:

```go
version, err := adapter.GetVersion(ctx)
if err != nil {
    // Handle error - could be network, auth, or API error
    log.Printf("Failed to get version: %v", err)
}
```

## Future Work

- WebSocket support for `/logs` endpoint
- Retry logic for transient failures
- Connection pooling optimization
- Metrics and observability

## Related Documentation

- [API Differences](../../docs/clashapi-diff.md) - Detailed comparison
- [sing-box Clash API](https://sing-box.sagernet.org/configuration/experimental/clash-api/)
- [mihomo Documentation](https://wiki.metacubex.one/)
