package store

import (
	"bytes"
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
				continue
			}
			keyBytes = k
		case collections.Prefix:
			prefixBytes := k.Bytes()
			if len(prefixBytes) == 0 {
				t.Fatalf("key %s has empty prefix", name)
				continue
			}
			keyBytes = prefixBytes
		default:
			t.Fatalf("unknown key type for %s: %T", name, key)
			continue
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

// CheckPrefixCollisions checks that no key is a prefix of another key.
// This ensures that keys used as storage prefixes don't collide.
func CheckPrefixCollisions(t *testing.T, keys map[string]interface{}) {
	t.Helper()

	keyList := make(map[string][]byte)

	for name, key := range keys {
		var keyBytes []byte
		switch k := key.(type) {
		case []byte:
			if len(k) == 0 {
				t.Fatalf("key %s has empty byte slice", name)
				continue
			}
			keyBytes = k
		case collections.Prefix:
			prefixBytes := k.Bytes()
			if len(prefixBytes) == 0 {
				t.Fatalf("key %s has empty prefix", name)
				continue
			}
			keyBytes = prefixBytes
		default:
			t.Fatalf("unknown key type for %s: %T", name, key)
			continue
		}

		keyList[name] = keyBytes
	}

	hasCollision := false
	for name1, key1 := range keyList {
		for name2, key2 := range keyList {
			if name1 >= name2 {
				continue
			}

			if bytes.HasPrefix(key1, key2) {
				hasCollision = true
				t.Errorf("PREFIX COLLISION: Key %s (0x%x) is a prefix of key %s (0x%x)", name2, key2, name1, key1)
			} else if bytes.HasPrefix(key2, key1) {
				hasCollision = true
				t.Errorf("PREFIX COLLISION: Key %s (0x%x) is a prefix of key %s (0x%x)", name1, key1, name2, key2)
			}
		}
	}

	if hasCollision {
		t.Fatal("Found prefix collisions")
	}
}
