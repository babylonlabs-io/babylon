package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/store"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

func TestNoKeyCollision(t *testing.T) {
	keys := map[string]interface{}{
		"ParamsKey":                   types.ParamsKey,
		"FinalityProviderKey":         types.FinalityProviderKey,
		"BTCDelegatorKey":             types.BTCDelegatorKey,
		"BTCDelegationKey":            types.BTCDelegationKey,
		"BTCHeightKey":                types.BTCHeightKey,
		"PowerDistUpdateKey":          types.PowerDistUpdateKey,
		"AllowedStakingTxHashesKey":   types.AllowedStakingTxHashesKey,
		"HeightToVersionMapKey":       types.HeightToVersionMapKey,
		"LargestBtcReorgInBlocks":     types.LargestBtcReorgInBlocks,
		"FpBbnAddrKey":                types.FpBbnAddrKey,
		"FinalityProvidersDeleted":    types.FinalityProvidersDeleted,
	}

	store.CheckKeyCollisions(t, keys)
}
