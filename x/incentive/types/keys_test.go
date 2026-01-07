package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/store"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

func TestNoKeyCollision(t *testing.T) {
	keys := map[string]interface{}{
		"ParamsKey":                                  types.ParamsKey,
		"BTCStakingGaugeKey":                         types.BTCStakingGaugeKey,
		"DelegatorWithdrawAddrPrefix":                types.DelegatorWithdrawAddrPrefix,
		"RewardGaugeKey":                             types.RewardGaugeKey,
		"RefundableMsgKeySetPrefix":                  types.RefundableMsgKeySetPrefix,
		"FinalityProviderCurrentRewardsKeyPrefix":    types.FinalityProviderCurrentRewardsKeyPrefix,
		"FinalityProviderHistoricalRewardsKeyPrefix": types.FinalityProviderHistoricalRewardsKeyPrefix,
		"BTCDelegationRewardsTrackerKeyPrefix":       types.BTCDelegationRewardsTrackerKeyPrefix,
		"BTCDelegatorToFPKey":                        types.BTCDelegatorToFPKey,
		"RewardTrackerEvents":                        types.RewardTrackerEvents,
		"RewardTrackerEventsLastProcessedHeight":     types.RewardTrackerEventsLastProcessedHeight,
		"FPDirectGaugeKey":                           types.FPDirectGaugeKey,
	}

	store.CheckKeyCollisions(t, keys)
}
