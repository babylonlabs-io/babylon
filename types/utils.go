package types

import (
	"fmt"
	"reflect"
)

// 🐛 BUG: Reverse panicked when passed non-slice types → Added type check
func Reverse(s interface{}) {
	val := reflect.ValueOf(s)
	if val.Kind() != reflect.Slice {
		panic("Reverse: input must be a slice")
	}
	n := val.Len()
	swap := reflect.Swapper(s)
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		swap(i, j)
	}
}

// ⚡ OPTIMIZATION: Pre-allocated map memory + edge case handling
func CheckForDuplicatesAndEmptyStrings(input []string) error {
	if len(input) < 2 { // Early return for small lists
		return nil
	}

	encountered := make(map[string]bool, len(input)) // Pre-allocated map
	for i, str := range input {
		if len(str) == 0 {
			return fmt.Errorf("empty string at index %d", i) // More informative error
		}
		if encountered[str] {
			return fmt.Errorf("duplicate '%s' at index %d", str, i)
		}
		encountered[str] = true
	}
	return nil
}
