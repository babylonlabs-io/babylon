package keeper

import (
	"fmt"
	"strings"

	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v3/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AddBsnRewards adds rewards for finality providers of a specific BSN consumer
func (k Keeper) AddBsnRewards(
	ctx sdk.Context,
	sender sdk.AccAddress,
	bsnConsumerId string,
	totalRewards sdk.Coins,
	fpRatios []types.FpRatio,
) error {
	// 1. Validate that sender has sufficient balance
	spendableCoins := k.bankKeeper.SpendableCoins(ctx, sender)
	if !spendableCoins.IsAllGTE(totalRewards) {
		return status.Errorf(codes.InvalidArgument, "insufficient balance: spendable %s and total rewards %s", spendableCoins.String(), totalRewards.String())
	}

	// 2. Transfer funds from sender to incentives module account
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, ictvtypes.ModuleName, totalRewards); err != nil {
		return types.ErrUnableToSendCoins.Wrapf("failed to send coins to incentive module account: %v", err)
	}

	// 3. Collects the babylon and the FP commission, and allocates the remaining rewards to btc stakers according to their voting power and fp ratio
	eventFpRewards, babylonCommission, err := k.CollectComissionAndDistributeBsnRewards(ctx, bsnConsumerId, totalRewards, fpRatios)
	if err != nil {
		return types.ErrUnableToDistributeBsnRewards.Wrapf("failed: %v", err)
	}

	// 4. Emit typed evt
	evt := &types.EventAddBsnRewards{
		Sender:            sender.String(),
		BsnConsumerId:     bsnConsumerId,
		TotalRewards:      totalRewards,
		BabylonCommission: babylonCommission,
		FpRatios:          eventFpRewards,
	}
	if err := ctx.EventManager().EmitTypedEvent(evt); err != nil {
		panic(fmt.Errorf("failed to emit EventAddBsnRewards event: %w", err))
	}

	return nil
}

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
		_, _, err := k.DistributeFpCommissionAndBtcDelRewards(ctx, bsnConsumerId, *fpRatio.BtcPk, fpRewards)
		if err != nil {
			return nil, nil, err
		}

		// 4. Collect event data
		evtFpRewards[i] = types.EventFpRewardInfo{
			FpBtcPkHex:   fpRatio.BtcPk.MarshalHex(),
			TotalRewards: fpRewards,
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
		return nil, nil, types.ErrConsumerIDNotRegistered.Wrapf("consumer %s: %v", bsnConsumerId, err)
	}

	// 2. Calculate Babylon commission
	babylonCommission = ictvtypes.GetCoinsPortion(totalRewards, consumerRegister.BabylonRewardsCommission)

	// 3. Calculate remaining rewards after Babylon commission
	remainingRewards = totalRewards.Sub(babylonCommission...)

	// 4. If there is no babylon commission, just returns
	if !babylonCommission.IsAllPositive() {
		return babylonCommission, remainingRewards, nil
	}

	// 5. Collect Babylon commission
	if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, ictvtypes.ModuleName, ictvtypes.ModAccCommissionCollectorBSN, babylonCommission); err != nil {
		return nil, nil, types.ErrUnableToSendCoins.Wrapf("failed to collect Babylon commission: %v", err)
	}

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
		return nil, nil, types.ErrFpNotFound.Wrapf("finality provider %s: %v", fpBtcPk.MarshalHex(), err)
	}

	// 2. Validate that FP belongs to the specified BSN consumer
	if !strings.EqualFold(fp.BsnId, bsnConsumerId) {
		return nil, nil, types.ErrFpInvalidBsnID.Wrapf("finality provider %s belongs to BSN consumer %s, not %s", fpBtcPk.MarshalHex(), fp.BsnId, bsnConsumerId)
	}

	// 3. Calculate FP commission
	fpCommission = ictvtypes.GetCoinsPortion(fpRewards, *fp.Commission)
	if fpCommission.IsAllPositive() {
		// 4. Add FP commission to existing reward gauge system via incentive module
		k.ictvKeeper.AccumulateRewardGaugeForFP(ctx, fp.Address(), fpCommission)
	}

	delegatorRewards = fpRewards.Sub(fpCommission...)
	if !delegatorRewards.IsAllPositive() {
		return fpCommission, delegatorRewards, nil
	}

	// 5. Remaining goes to BTC delegations via existing F1 system
	if err := k.ictvKeeper.AddFinalityProviderRewardsForBtcDelegations(ctx, fp.Address(), delegatorRewards); err != nil {
		return nil, nil, types.ErrUnableToAllocateBtcRewards.Wrapf("failed: %v", err)
	}

	return fpCommission, delegatorRewards, nil
}
