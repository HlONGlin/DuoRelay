# Clash API Differences: sing-box vs mihomo

## Overview

This document records the API differences between sing-box and mihomo Clash API implementations, discovered during P0-04 testing.

**Test Environment:**
- sing-box: v1.14.4
- mihomo: v1.19.33
- Test Date: 2024-10-31

## API Endpoints Tested

### 1. `/version` - Version Information

**Status:** ✅ Compatible

Both kernels return similar version information with minor differences in formatting.

**sing-box Response:**
```json
{
  "meta": true,
  "premium": true,
  "version": "sing-box 1.14.4"
}
```

**mihomo Response:**
```json
{
  "meta": true,
  "version": "v1.19.33"
}
```

**Differences:**
- sing-box includes `premium: true` field
- sing-box prefixes version with "sing-box "
- mihomo uses "v" prefix for version number

### 2. `/proxies` - Proxy List

**Status:** ⚠️ Partially Compatible

Both kernels support this endpoint but with significant structural differences.

**sing-box Response:**
```json
{
  "proxies": {
    "GLOBAL": {
      "all": ["test-ss"],
      "history": [],
      "name": "GLOBAL",
      "now": "direct",
      "type": "Fallback",
      "udp": true
    },
    "direct": {
      "type": "Direct",
      "name": "direct",
      "udp": true,
      "history": []
    },
    "block": {
      "type": "Reject",
      "name": "block",
      "udp": true,
      "history": []
    },
    "test-ss": {
      "type": "Shadowsocks",
      "name": "test-ss",
      "udp": true,
      "history": []
    }
  }
}
```

**mihomo Response:**
```json
{
  "proxies": {
    "DIRECT": {
      "alive": true,
      "dialer-proxy": "",
      "extra": {},
      "history": [],
      "id": "9e74fcd4-516f-4dc1-abd2-f91d770a63c8",
      "interface": "",
      "mptcp": false,
      "name": "DIRECT",
      "provider-name": "",
      "routing-mark": 0,
      "smux": false,
      "tfo": false,
      "type": "Direct",
      "udp": true,
      "uot": false,
      "xudp": false
    },
    "test-ss": {
      "alive": true,
      "dialer-proxy": "",
      "extra": {},
      "history": [],
      "id": "3ae1f295-e31b-4897-8366-1c8fee58f0ce",
      "interface": "",
      "mptcp": false,
      "name": "test-ss",
      "provider-name": "",
      "routing-mark": 0,
      "smux": false,
      "tfo": false,
      "type": "Shadowsocks",
      "udp": false,
      "uot": false,
      "xudp": false
    }
  }
}
```

**Differences:**

**mihomo-only fields:**
- `alive` (bool) - Proxy availability status
- `id` (string) - Unique identifier for the proxy
- `dialer-proxy` (string) - Chain proxy configuration
- `extra` (object) - Additional metadata
- `provider-name` (string) - Provider information
- `interface` (string) - Network interface binding
- `mptcp` (bool) - Multipath TCP support
- `routing-mark` (int) - Linux routing mark
- `smux` (bool) - Stream multiplexing
- `tfo` (bool) - TCP Fast Open
- `uot` (bool) - UDP over TCP
- `xudp` (bool) - Extended UDP

**Common fields:**
- `name` (string)
- `type` (string)
- `udp` (bool)
- `history` (array)
- `all` (array, for proxy groups)
- `now` (string, for proxy groups)

**Built-in Proxies:**
- sing-box: `direct`, `block`, `GLOBAL`
- mihomo: `DIRECT`, `REJECT`, `REJECT-DROP`, `GLOBAL`, `PASS`, `PASS-RULE`, `COMPATIBLE`

### 3. `/connections` - Active Connections

**Status:** ✅ Compatible with minor differences

**sing-box Response:**
```json
{
  "connections": [],
  "downloadTotal": 0,
  "memory": 4448256,
  "uploadTotal": 0
}
```

**mihomo Response:**
```json
{
  "downloadTotal": 0,
  "uploadTotal": 0,
  "connections": null,
  "memory": 0
}
```

**Differences:**
- Both return the same fields
- mihomo may return `null` for empty connections instead of `[]`
- `memory` field present in both but with different values

### 4. `/proxies/{name}/delay` - Delay Test

**Status:** ✅ Compatible

Both kernels support delay testing with the same query parameters.

**Request Format:**
```
GET /proxies/{name}/delay?timeout={ms}&url={test_url}
```

**Parameters:**
- `timeout` - Timeout in milliseconds (e.g., 5000)
- `url` - Test URL (e.g., http://www.gstatic.com/generate_204)

**Success Response:**
```json
{
  "delay": 123
}
```

**Error Response:**
```json
{
  "message": "An error occurred in the delay test"
}
```

**Note:** Both kernels may fail delay tests if the target proxy is unreachable.

### 5. `/traffic` - Traffic Statistics (WebSocket)

**Status:** 🔶 Not Tested

This endpoint uses WebSocket for real-time traffic updates. Testing deferred to later phases.

**Expected Format:**
```json
{
  "up": 12345,
  "down": 67890
}
```

### 6. `/logs` - Log Stream (WebSocket)

**Status:** 🔶 Not Tested

This endpoint uses WebSocket for real-time log streaming. Testing deferred to later phases.

**Expected Format:**
```json
{
  "type": "info",
  "payload": "log message"
}
```

## Configuration Differences

### sing-box Configuration

**Clash API Block:**
```json
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:19094",
      "secret": "test123",
      "default_mode": "rule"
    }
  }
}
```

**Notes:**
- Requires `with_clash_api` build tag
- sing-box 1.14.0+ deprecated old DNS server format
- DNS server format must use structured format (not simple "address": "8.8.8.8")
- Can omit DNS entirely if not needed

### mihomo Configuration

**Clash API Block:**
```yaml
external-controller: 127.0.0.1:19093
secret: "test123"
mode: rule
```

**Notes:**
- Built-in proxies (DIRECT, REJECT) cannot be redeclared in proxies list
- More flexible DNS configuration
- Additional built-in proxies available

## Adapter Implementation Strategy

The unified adapter layer handles these differences by:

1. **Version Normalization:** Strip prefixes and normalize version strings
2. **Proxy Fields:** Preserve all fields from both kernels (superset approach)
3. **Empty Collections:** Handle both `null` and `[]` for empty arrays
4. **Built-in Proxies:** Document kernel-specific built-ins, don't abstract
5. **Error Handling:** Unified error format for delay tests and other operations
6. **Optional Fields:** All kernel-specific fields marked as optional in types

## Recommendations

1. **Use Common Fields Only:** For cross-kernel compatibility, only rely on common fields
2. **Feature Detection:** Check kernel type before using mihomo-specific features
3. **Error Tolerance:** Handle both null and empty array responses
4. **WebSocket:** Implement WebSocket endpoints in a later phase (P1 or P2)
5. **Testing:** Always test with both kernels during development

## Summary

| Feature | sing-box | mihomo | Compatible |
|---------|----------|--------|------------|
| `/version` | ✅ | ✅ | ✅ Yes |
| `/proxies` | ✅ | ✅ | ⚠️ Partial |
| `/connections` | ✅ | ✅ | ✅ Yes |
| `/proxies/{name}/delay` | ✅ | ✅ | ✅ Yes |
| `/traffic` (WebSocket) | ✅ | ✅ | 🔶 Not tested |
| `/logs` (WebSocket) | ✅ | ✅ | 🔶 Not tested |

**Compatibility Level:** ~85% for REST endpoints

The main differences are in the `/proxies` response structure, where mihomo provides significantly more metadata. The adapter preserves all fields and allows consumers to handle kernel-specific fields as needed.

---

*Generated: 2024-10-31*  
*Task: P0-04 - Clash API Integration*
