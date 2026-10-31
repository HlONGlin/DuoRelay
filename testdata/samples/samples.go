package testdata

// This package contains sample data for testing parsers and generators

const (
	// Sample Shadowsocks share links
	ShadowsocksLink1 = "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388#Test%20Node"
	ShadowsocksLink2 = "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@192.168.1.1:8388"

	// Sample VMess share links
	VMessLink1 = "vmess://eyJhZGQiOiJleGFtcGxlLmNvbSIsImFpZCI6IjAiLCJob3N0IjoiIiwiaWQiOiI0MTgwNDhhZi1hMjkzLTRiOTktOWIwYy05OGNhMzU4MGRkMjQiLCJuZXQiOiJ0Y3AiLCJwYXRoIjoiIiwicG9ydCI6IjEwMDg2IiwicHMiOiJWTWVzcyBUZXN0Iiwic2N5IjoiYXV0byIsInNuaSI6IiIsInRscyI6IiIsInR5cGUiOiJub25lIiwidiI6IjIifQ=="

	// Sample VLESS share links
	VLESSLink1 = "vless://418048af-a293-4b99-9b0c-98ca3580dd24@example.com:443?encryption=none&security=tls&sni=example.com&type=tcp#VLESS%20Test"

	// Sample Trojan share links
	TrojanLink1 = "trojan://password@example.com:443?sni=example.com#Trojan%20Test"

	// Sample subscription responses (Base64 encoded)
	SubscriptionBase64 = "c3M6Ly9ZV1Z6TFRJMU5pMW5ZMjA2Y0dGemMzZHZjbVJBWlhoaGJYQnNaUzVqYjIwNk9ETTQ0ZzBLdm1lc2N6b3ZMMlY1U25wYVdFcHdZbTA1YWFWSWJHeGlVemwyV2xOSmFWb3lPWFZqUkc5cFRFTktjRnBYUm5CaVF6bDBZa1ZPYlZveWJHdGlRVDA5"
)

// ProtocolSamples contains test samples for each protocol
var ProtocolSamples = map[string][]string{
	"shadowsocks": {
		ShadowsocksLink1,
		ShadowsocksLink2,
	},
	"vmess": {
		VMessLink1,
	},
	"vless": {
		VLESSLink1,
	},
	"trojan": {
		TrojanLink1,
	},
	// More protocols will be added during P1-05~P2-04
}

// GetSample returns a sample for the given protocol and index
func GetSample(protocol string, index int) string {
	samples, ok := ProtocolSamples[protocol]
	if !ok || index >= len(samples) {
		return ""
	}
	return samples[index]
}
