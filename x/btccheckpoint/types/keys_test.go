package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/store"
	"github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
)

func TestNoKeyCollision(t *testing.T) {
	keys := map[string]interface{}{
		"SubmisionKeyPrefix":       types.SubmisionKeyPrefix,
		"EpochDataPrefix":          types.EpochDataPrefix,
		"LastFinalizedEpochKey":    types.LastFinalizedEpochKey,
		"BtcLightClientUpdatedKey": types.BtcLightClientUpdatedKey,
		"ParamsKey":                types.ParamsKey,
	}

	store.CheckKeyCollisions(t, keys)
}
