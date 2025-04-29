package types

import "fmt"

// Validator is an interface for types that have a Validate method.
type Validator interface {
	Validate() error
}

// ValidateEntries checks for duplicates based on a key extracted by keyFunc and validates each entry.
func ValidateEntries[T Validator, K comparable](entries []T, keyFunc func(T) K) error {
	keyMap := make(map[K]bool)
	for _, entry := range entries {
		key := keyFunc(entry)
		if _, exists := keyMap[key]; exists {
			return fmt.Errorf("duplicate entry for key: %v", key)
		}
		keyMap[key] = true

		if err := entry.Validate(); err != nil {
			return err
		}
	}
	return nil
}
