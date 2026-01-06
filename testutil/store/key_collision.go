package store

import (
	"testing"

	"cosmossdk.io/collections"
)

// CheckKeyCollisions checks that all provided keys use unique prefixes.
// Returns a map of prefix -> key names for debugging.
func CheckKeyCollisions(t *testing.T, keys map[string]interface{}) map[byte][]string {
	t.Helper()

	prefixMap := make(map[byte][]string)

	for name, key := range keys {
		var prefix byte
		switch k := key.(type) {
		case []byte:
			if len(k) > 0 {
				prefix = k[0]
			} else {
				t.Fatalf("key %s has empty byte slice", name)
				continue
			}
		case collections.Prefix:
			prefixBytes := k.Bytes()
			if len(prefixBytes) > 0 {
				prefix = prefixBytes[0]
			} else {
				t.Fatalf("key %s has empty prefix", name)
				continue
			}
		default:
			t.Fatalf("unknown key type for %s: %T", name, key)
			continue
		}

		prefixMap[prefix] = append(prefixMap[prefix], name)
	}

	hasCollision := false
	for prefix, keyNames := range prefixMap {
		if len(keyNames) > 1 {
			hasCollision = true
			t.Errorf("KEY COLLISION: Prefix 0x%02x (%d decimal) is used by multiple keys: %v", prefix, prefix, keyNames)
		}
	}

	if hasCollision {
		t.Fatal("Found key collisions")
	}

	return prefixMap
}
