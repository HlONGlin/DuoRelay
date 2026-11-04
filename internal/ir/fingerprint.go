package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// ConnectionFingerprint excludes presentation and ownership metadata. Native
// passthrough options remain part of identity so distinct connections survive.
func ConnectionFingerprint(n *Node) string {
	if n == nil {
		return ""
	}
	extra := make(map[string]any, len(n.Extra))
	for key, value := range n.Extra {
		if entry, ok := value.(map[string]any); ok {
			clean := make(map[string]any, len(entry))
			for field, option := range entry {
				switch field {
				case "name", "tag", "ps", "remarks":
					continue
				}
				clean[field] = option
			}
			extra[key] = clean
		} else {
			extra[key] = value
		}
	}
	server := n.Server
	server.Host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(server.Host), "."))
	data, _ := json.Marshal([]any{n.Protocol, server, n.Transport, n.Security, n.Proto, extra})
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
