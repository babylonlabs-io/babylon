package keeper

import (
	"fmt"
	"strings"

	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v3/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CollectComissionAndDistributeBsnRewards collects the commissions for babylon and FPs and
// allocates rewards to the BTC stakers based on the FP ratios
// 1. Collect babylon commission
// [for each fp]
//  2. Calculate the FP entitled rewards based on the ratio
//  3. Validate FP BSN ID matches the bsnConsumerId
//  4. Collect FP commission
//  5. Allocate the remaining funds to the BTC stakers of that FP
func (k Keeper) CollectComissionAndDistributeBsnRewards(
	ctx sdk.Context,
	bsnConsumerId string,
	totalRewards sdk.Coins,
	fpRatios []types.FpRatio,
) (evtFpRatios []types.EventFpRewardInfo, bbnCommission sdk.Coins, err error) {
	// 1. Calculate and collect Babylon commission
	babylonCommission, remainingRewards, err := k.CollectBabylonCommission(ctx, bsnConsumerId, totalRewards)
	if err != nil {
		return nil, nil, err
	}

	evtFpRewards := make([]types.EventFpRewardInfo, len(fpRatios))

	for i, fpRatio := range fpRatios {
		// 2. Calculate this FP's total allocation from remaining rewards
		fpRewards := ictvtypes.GetCoinsPortion(remainingRewards, fpRatio.Ratio)

		// 3. Distribute FP commission and delegator rewards
		fpCommission, delegatorRewards, err := k.DistributeFpCommissionAndBtcDelRewards(ctx, bsnConsumerId, *fpRatio.BtcPk, fpRewards)
		if err != nil {
			return nil, nil, err
		}

		// 4. Collect event data
		evtFpRewards[i] = types.EventFpRewardInfo{
			BtcPk:            fpRatio.BtcPk,
			Ratio:            fpRatio.Ratio,
			TotalAllocated:   fpRewards,
			FpCommission:     fpCommission,
			DelegatorRewards: delegatorRewards,
		}
	}

	return evtFpRewards, babylonCommission, nil
}

// CollectBabylonCommission calculates and collects Babylon commission
// based on the commission set in the BSN consumer registry, sends it to the
// defined module account and reduce it from total rewards
func (k Keeper) CollectBabylonCommission(
	ctx sdk.Context,
	bsnConsumerId string,
	totalRewards sdk.Coins,
) (babylonCommission sdk.Coins, remainingRewards sdk.Coins, err error) {
	// 1. Get BSN consumer register and validate
	consumerRegister, err := k.BscKeeper.GetConsumerRegister(ctx, bsnConsumerId)
	if err != nil {
		return nil, nil, fmt.Errorf("BSN consumer not found: %w", err)
	}

	// 2. Calculate and collect Babylon commission
	babylonCommission = ictvtypes.GetCoinsPortion(totalRewards, consumerRegister.BabylonRewardsCommission)
	if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, ictvtypes.ModuleName, ictvtypes.ModAccCommissionCollectorBSN, babylonCommission); err != nil {
		return nil, nil, fmt.Errorf("failed to collect Babylon commission: %w", err)
	}

	// 3. Calculate remaining rewards after Babylon commission
	remainingRewards = totalRewards.Sub(babylonCommission...)

	return babylonCommission, remainingRewards, nil
}

// DistributeFpCommissionAndBtcDelRewards distributes rewards for a single finality provider,
// splitting between FP commission and BTC delegators rewards
func (k Keeper) DistributeFpCommissionAndBtcDelRewards(
	ctx sdk.Context,
	bsnConsumerId string,
	fpBtcPk bbntypes.BIP340PubKey,
	fpRewards sdk.Coins,
) (fpCommission sdk.Coins, delegatorRewards sdk.Coins, err error) {
	// 1. Get finality provider for commission rate
	fp, err := k.GetFinalityProvider(ctx, fpBtcPk.MustMarshal())
	if err != nil {
		return nil, nil, fmt.Errorf("finality provider not found: %w", err)
	}

	// 2. Validate that FP belongs to the specified BSN consumer
	if !strings.EqualFold(fp.BsnId, bsnConsumerId) {
		return nil, nil, fmt.Errorf("finality provider %s belongs to BSN consumer %s, not %s", fpBtcPk.MarshalHex(), fp.BsnId, bsnConsumerId)
	}

	// 3. Calculate FP commission
	fpCommission = ictvtypes.GetCoinsPortion(fpRewards, *fp.Commission)

	// 4. Add FP commission to existing reward gauge system via incentive module
	k.ictvKeeper.AccumulateRewardGaugeForFP(ctx, fp.Address(), fpCommission)

	// 5. Remaining goes to BTC delegations via existing F1 system
	delegatorRewards = fpRewards.Sub(fpCommission...)
	if err := k.ictvKeeper.AddFinalityProviderRewardsForBtcDelegations(ctx, fp.Address(), delegatorRewards); err != nil {
		return nil, nil, fmt.Errorf("failed to add delegator rewards: %w", err)
	}

	return fpCommission, delegatorRewards, nil
}
