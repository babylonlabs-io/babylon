package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/store"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
)

func TestNoKeyCollision(t *testing.T) {
	keys := map[string]interface{}{
		"EpochInfoKey":           types.EpochInfoKey,
		"QueueLengthKey":         types.QueueLengthKey,
		"MsgQueueKey":            types.MsgQueueKey,
		"ValidatorSetKey":        types.ValidatorSetKey,
		"VotingPowerKey":         types.VotingPowerKey,
		"SlashedVotingPowerKey":  types.SlashedVotingPowerKey,
		"SlashedValidatorSetKey": types.SlashedValidatorSetKey,
		"ValidatorLifecycleKey":  types.ValidatorLifecycleKey,
		"DelegationLifecycleKey": types.DelegationLifecycleKey,
		"ParamsKey":              types.ParamsKey,
	}

	store.CheckKeyCollisions(t, keys)
}
