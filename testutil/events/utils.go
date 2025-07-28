package events

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/test-go/testify/require"
)

func RequireEventAttribute(t *testing.T, event sdk.Event, key, expectedValue string, msgAndArgs ...any) {
	t.Helper()
	for _, attr := range event.Attributes {
		if attr.Key == key {
			require.Equal(t, expectedValue, attr.Value, msgAndArgs...)
			return
		}
	}
	require.Fail(t, "Expected attribute not found", msgAndArgs...)
}
