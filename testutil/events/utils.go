package events

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
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

func IsEventType(event abci.Event, eventMsg sdk.Msg) bool {
	return event.Type == sdk.MsgTypeURL(eventMsg) || "/"+event.Type == sdk.MsgTypeURL(eventMsg)
}
