package store

import (
	"fmt"
	"testing"

	"cosmossdk.io/collections"
)

// CheckKeyCollisions checks that all provided keys are unique.
// For composite keys (multi-byte), it compares the full key.
// Returns a map of key bytes -> key names for debugging.
func CheckKeyCollisions(t *testing.T, keys map[string]interface{}) map[string][]string {
	t.Helper()

	keyMap := make(map[string][]string)

	for name, key := range keys {
		var keyBytes []byte
		switch k := key.(type) {
		case []byte:
			if len(k) == 0 {
				t.Fatalf("key %s has empty byte slice", name)
			}
			keyBytes = k
		case collections.Prefix:
			prefixBytes := k.Bytes()
			if len(prefixBytes) == 0 {
				t.Fatalf("key %s has empty prefix", name)
			}
			keyBytes = prefixBytes
		default:
			t.Fatalf("unknown key type for %s: %T", name, key)
		}

		keyStr := fmt.Sprintf("%x", keyBytes)
		keyMap[keyStr] = append(keyMap[keyStr], name)
	}

	hasCollision := false
	for keyStr, keyNames := range keyMap {
		if len(keyNames) > 1 {
			hasCollision = true
			t.Errorf("KEY COLLISION: Key 0x%s is used by multiple keys: %v", keyStr, keyNames)
		}
	}

	if hasCollision {
		t.Fatal("Found key collisions")
	}

	return keyMap
}
