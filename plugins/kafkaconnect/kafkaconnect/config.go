package kafkaconnect

import (
	"sort"
	"strings"
)

// connectorConfig is a connector's config map with the accessors the
// resolvers need. Kafka Connect stores every value as a string.
type connectorConfig map[string]string

// get returns the first non-empty value among keys.
func (c connectorConfig) get(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(c[key]); v != "" {
			return v
		}
	}
	return ""
}

// list splits the first non-empty value among keys on commas.
func (c connectorConfig) list(keys ...string) []string {
	return splitList(c.get(keys...))
}

// class is the connector class, as configured.
func (c connectorConfig) class() string {
	return c.get("connector.class")
}

// isTrue reports whether a config flag is set to true.
func (c connectorConfig) isTrue(key string) bool {
	return strings.EqualFold(c.get(key), "true")
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// pairs parses a "key:value,key:value" mapping, the shape connectors
// use for topic-to-table maps.
func pairs(value string) map[string]string {
	out := make(map[string]string)
	for _, entry := range splitList(value) {
		key, val, ok := strings.Cut(entry, ":")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if key != "" && val != "" {
			out[key] = val
		}
	}
	return out
}

// secretKeyFragments are the substrings that mark a config key as
// holding a credential. Any key containing one is masked before the
// config is stored as metadata.
var secretKeyFragments = []string{
	"password", "secret", "token", "credential", "sasl.jaas", "key.id", "access.key", "private",
}

const maskedValue = "****"

func isSecretKey(key string) bool {
	lower := strings.ToLower(key)
	for _, fragment := range secretKeyFragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

// sanitiseConfig copies a connector config with every credential value
// masked, so the stored metadata never carries a secret.
func sanitiseConfig(config map[string]string) map[string]any {
	out := make(map[string]any, len(config))
	for key, value := range config {
		if isSecretKey(key) {
			out[key] = maskedValue
			continue
		}
		out[key] = value
	}
	return out
}

// dedupe returns the non-empty values of in, once each, sorted.
func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// lastSegment returns the part of a dotted identifier after the last
// dot, which is the bare object name.
func lastSegment(identifier string) string {
	return identifier[strings.LastIndex(identifier, ".")+1:]
}

// literalIdentifier turns an include-list entry into a plain identifier.
// Debezium include lists are regular expressions, so an entry that uses
// anything beyond escaped dots names a pattern rather than one object
// and cannot be resolved to a dataset.
func literalIdentifier(entry string) (string, bool) {
	unescaped := strings.ReplaceAll(strings.TrimSpace(entry), `\.`, ".")
	if unescaped == "" || strings.ContainsAny(unescaped, `\^$*+?()[]{}|`) {
		return "", false
	}
	return unescaped, true
}
