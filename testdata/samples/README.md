# DuoRelay Test Samples Library

## Overview

This directory contains sanitized test samples for various proxy protocols supported by DuoRelay.

## Protocol Coverage

### A-Tier Protocols (Priority)
- ✅ Shadowsocks (3 samples)
- ✅ VMess (3 samples)
- ✅ VLESS (3 samples)
- ✅ Trojan (3 samples)
- ✅ Hysteria2 (3 samples)
- ✅ TUIC (3 samples)

**Total**: 18 samples for 6 protocols

## Sample Format

Each sample includes:
- `name`: Sample identifier
- `format`: URI or config format
- `link`: Share link or configuration
- `description`: Brief description
- `expected_ir`: Expected intermediate representation

## Privacy & Security

**All samples are sanitized**:
- ✅ Server addresses use `example.com`, `test.example.com`
- ✅ Passwords are test values (`test-password`, etc.)
- ✅ UUIDs are generated test values
- ✅ No real server information included

## Usage

### For Parser Testing
```go
import "github.com/holi/duorelay/testdata/samples"

func TestParser(t *testing.T) {
    sample := samples.LoadShadowsocksSamples()[0]
    node, err := parser.Parse(sample.Link)
    // Compare with sample.ExpectedIR
}
```

### For Generator Testing
```go
func TestGenerator(t *testing.T) {
    sample := samples.LoadVMessSamples()[0]
    node := sample.ExpectedIR
    config := generator.Generate([]*ir.Node{node})
    // Validate config
}
```

## Sample Variants

Each protocol includes variants covering:
1. **Basic**: Standard configuration with minimal options
2. **Transport**: Different transport layers (TCP, WebSocket, gRPC)
3. **Security**: Different security options (TLS, Reality, obfuscation)

## File Structure

```
testdata/samples/
├── README.md (this file)
├── shadowsocks_samples.json
├── vmess_samples.json
├── vless_samples.json
├── trojan_samples.json
├── hysteria2_samples.json
├── tuic_samples.json
└── subscription_samples/
    ├── base64_sample.txt (future)
    ├── clash_yaml_sample.yaml (future)
    └── singbox_json_sample.json (future)
```

## Quality Standards

All samples must:
- ✅ Use sanitized data (no real credentials)
- ✅ Include expected IR for validation
- ✅ Represent real-world configurations
- ✅ Be parseable by standard tools
- ✅ Cover edge cases and variants

## Adding New Samples

When adding new samples:
1. Ensure privacy: use `example.com` domains
2. Test the sample can be parsed
3. Include expected IR output
4. Add description explaining the variant
5. Update this README

## Testing

Validate all samples:
```bash
go test ./testdata/samples/...
```

## Future Expansion

Additional protocols to be added in P1:
- SOCKS5
- HTTP/HTTPS
- SSH
- Hysteria (v1)
- Snell
- ... (total 28 protocols)

---

**Status**: P0-09 Complete  
**Coverage**: 6/6 A-tier protocols  
**Sample Count**: 18 total  
**Last Updated**: 2024-10-31
