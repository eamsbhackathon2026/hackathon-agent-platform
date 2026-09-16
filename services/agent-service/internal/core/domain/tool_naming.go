package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// UniqueToolName returns a provider-safe, deterministic name for one saved source.
func UniqueToolName(prefix, raw, identity string, used map[string]bool) string {
	base := sanitizeToolName(prefix + raw)
	if len(base) > 64 {
		base = strings.TrimRight(base[:64], "_")
	}
	if base != "" && !used[base] {
		used[base] = true
		return base
	}
	sum := sha256.Sum256([]byte(identity))
	suffix := "_" + hex.EncodeToString(sum[:3])
	limit := 64 - len(suffix)
	if len(base) > limit {
		base = strings.TrimRight(base[:limit], "_")
	}
	if base == "" {
		base = "tool"
	}
	name := base + suffix
	for used[name] {
		sum = sha256.Sum256([]byte(identity + name))
		name = base + "_" + hex.EncodeToString(sum[:3])
	}
	used[name] = true
	return name
}

func sanitizeToolName(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
	}
	return strings.Trim(builder.String(), "_-")
}
