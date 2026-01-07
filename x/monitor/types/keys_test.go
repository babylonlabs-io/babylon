package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/store"
	"github.com/babylonlabs-io/babylon/v4/x/monitor/types"
)

func TestNoKeyCollision(t *testing.T) {
	keys := map[string]interface{}{
		"EpochEndLightClientHeightPrefix":           types.EpochEndLightClientHeightPrefix,
		"CheckpointReportedLightClientHeightPrefix": types.CheckpointReportedLightClientHeightPrefix,
	}

	store.CheckKeyCollisions(t, keys)
}
