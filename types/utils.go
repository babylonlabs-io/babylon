package types

import (
	"fmt"
	"reflect"
)

func Reverse(s interface{}) {
	n := reflect.ValueOf(s).Len()
	swap := reflect.Swapper(s)
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		swap(i, j)
	}
}

func CheckForDuplicatesAndEmptyStrings(input []string) error {
	encountered := map[string]bool{}
	for _, str := range input {
		if len(str) == 0 {
			return fmt.Errorf("empty string is not allowed")
		}

		if encountered[str] {
			return fmt.Errorf("duplicate entry found: %s", str)
		}

		encountered[str] = true
	}

	return nil
}

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
