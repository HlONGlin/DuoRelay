# Test Data Samples

This directory contains sample proxy share links and subscription data for testing parsers.

## Guidelines

1. **Privacy**: All samples MUST be sanitized - no real server addresses or credentials
2. **Coverage**: Each protocol should have at least 3 samples covering:
   - Standard format
   - Common variations (e.g., Base64 padding differences)
   - Edge cases (missing optional fields, extra fields)

3. **Organization**: 
   - Common samples in `samples.go`
   - Protocol-specific samples in `<protocol>_samples.json`
   - Subscription samples in `subscription_samples/`

## Sample Format

For structured samples, use JSON:

```json
{
  "protocol": "shadowsocks",
  "samples": [
    {
      "name": "standard",
      "link": "ss://...",
      "expected": {
        "server": "example.com",
        "port": 8388,
        "method": "aes-256-gcm"
      }
    }
  ]
}
```

## Status

- ✅ Shadowsocks: 2 samples
- ✅ VMess: 1 sample  
- ✅ VLESS: 1 sample
- ✅ Trojan: 1 sample
- ⏳ Other protocols: To be added during P1-05~P2-04

---

*Note: This directory is initialized in P0-09. Full sample library will be built incrementally.*
