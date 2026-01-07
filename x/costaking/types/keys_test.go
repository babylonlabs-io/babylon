package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/store"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
)

func TestNoKeyCollision(t *testing.T) {
	keys := map[string]interface{}{
		"ParamsKey":                       types.ParamsKey,
		"HistoricalRewardsKeyPrefix":      types.HistoricalRewardsKeyPrefix,
		"CurrentRewardsKeyPrefix":         types.CurrentRewardsKeyPrefix,
		"CostakerRewardsTrackerKeyPrefix": types.CostakerRewardsTrackerKeyPrefix,
		"ValidatorsKeyPrefix":             types.ValidatorsKeyPrefix,
	}

	store.CheckKeyCollisions(t, keys)
}
