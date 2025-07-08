package keeper

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcckpttypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

type DelegationTimeRangeInfo struct {
	StartHeight uint32
	EndHeight   uint32
	TipHeight   uint32
}

// VerifyInclusionProofAndGetHeight verifies the inclusion proof of the given staking tx
// and returns the start height and end height
// Note: the `minUnbondingTime` passed here should be from the corresponding params
// of the staking tx
func (k Keeper) VerifyInclusionProofAndGetHeight(
	ctx sdk.Context,
	stakingTx *btcutil.Tx,
	confirmationDepth uint32,
	stakingTime uint32,
	unbondingTime uint32,
	inclusionProof *types.ParsedProofOfInclusion,
) (*DelegationTimeRangeInfo, error) {
	// Check:
	// - timelock of staking tx
	// - staking tx is k-deep
	// - staking tx inclusion proof
	stakingTxHeader, err := k.btclcKeeper.GetHeaderByHash(ctx, inclusionProof.HeaderHash)
	if err != nil {
		return nil, fmt.Errorf("staking tx inclusion proof header %s is not found in BTC light client state: %v", inclusionProof.HeaderHash.MarshalHex(), err)
	}

	// no need to do more validations to the btc header as it was already
	// validated by the btclightclient module
	btcHeader := stakingTxHeader.Header.ToBlockHeader()

	proofValid := btcckpttypes.VerifyInclusionProof(
		stakingTx,
		&btcHeader.MerkleRoot,
		inclusionProof.Proof,
		inclusionProof.Index,
	)

	if !proofValid {
		return nil, types.ErrInvalidStakingTx.Wrapf("not included in the Bitcoin chain")
	}

	if inclusionProof.Index == 0 {
		return nil, types.ErrInvalidStakingTx.Wrapf("coinbase tx cannot be used for staking")
	}

	startHeight := stakingTxHeader.Height
	endHeight := stakingTxHeader.Height + stakingTime

	btcTip := k.btclcKeeper.GetTipInfo(ctx)
	stakingTxDepth := btcTip.Height - stakingTxHeader.Height
	if stakingTxDepth < confirmationDepth {
		return nil, types.ErrInvalidStakingTx.Wrapf("not k-deep: k=%d; depth=%d", confirmationDepth, stakingTxDepth)
	}

	// ensure staking tx's timelock has more than unbonding BTC blocks left
	if btcTip.Height+unbondingTime >= endHeight {
		return nil, types.ErrInvalidStakingTx.
			Wrapf("staking tx's timelock has no more than unbonding(=%d) blocks left", unbondingTime)
	}

	return &DelegationTimeRangeInfo{
		StartHeight: startHeight,
		EndHeight:   endHeight,
		TipHeight:   btcTip.Height,
	}, nil
}

// AddBTCDelegationInclusionProof adds the inclusion proof of the given
// BTC delegation realizing checks and updates in the store.
// 1. The given delegation and inclusion proof are not nil
// 2. The btcDel doesn't already have inclusion proof
// 3. Has enough covenant votes
// 4. It is not unbonded
// 5. Verify inclusion proof
// 6. The BTC start height of the BTC tx inclusion is higher or equal the informed tip of the btc del
// 7. Updates start and end height
// 8. Emit active event
func (k Keeper) AddBTCDelegationInclusionProof(
	ctx sdk.Context,
	btcDel *types.BTCDelegation,
	stakingTxInclusionProof *types.InclusionProof,
) error {
	// 1. sanity check the given params
	if btcDel == nil {
		return types.ErrBTCDelegationNotFound
	}
	if stakingTxInclusionProof == nil {
		return errors.New("nil inclusion proof")
	}

	stakingTxHash, err := btcDel.GetStakingTxHash()
	if err != nil {
		return err
	}
	stakingTxHashStr := stakingTxHash.String()

	// 2. check if the delegation already has inclusion proof
	if btcDel.HasInclusionProof() {
		return fmt.Errorf("the delegation %s already has inclusion proof", stakingTxHashStr)
	}

	params := k.GetParamsByVersion(ctx, btcDel.ParamsVersion)
	if params == nil {
		panic("params version in BTC delegation is not found")
	}

	// 3. check if the delegation has received a quorum of covenant sigs
	hasQuorum, err := k.BtcDelHasCovenantQuorums(ctx, btcDel, params.CovenantQuorum)
	if err != nil {
		return err
	}
	if !hasQuorum {
		return fmt.Errorf("the delegation %s has not received a quorum of covenant signatures", stakingTxHashStr)
	}

	// 4. check if the delegation is already unbonded
	if btcDel.BtcUndelegation.DelegatorUnbondingInfo != nil {
		return fmt.Errorf("the delegation %s is already unbonded", stakingTxHashStr)
	}

	// 5. verify inclusion proof
	parsedInclusionProof, err := types.NewParsedProofOfInclusion(stakingTxInclusionProof)
	if err != nil {
		return err
	}
	stakingTx, err := bbn.NewBTCTxFromBytes(btcDel.StakingTx)
	if err != nil {
		return err
	}

	btccParams := k.btccKeeper.GetParams(ctx)

	timeInfo, err := k.VerifyInclusionProofAndGetHeight(
		ctx,
		btcutil.NewTx(stakingTx),
		btccParams.BtcConfirmationDepth,
		btcDel.StakingTime,
		params.UnbondingTimeBlocks,
		parsedInclusionProof,
	)

	if err != nil {
		return fmt.Errorf("invalid inclusion proof: %w", err)
	}

	// 6. check if the staking tx is included after the BTC tip height at the time of the delegation creation
	if timeInfo.StartHeight < btcDel.BtcTipHeight {
		return types.ErrStakingTxIncludedTooEarly.Wrapf(
			"btc tip height at the time of the delegation creation: %d, staking tx inclusion height: %d",
			btcDel.BtcTipHeight,
			timeInfo.StartHeight,
		)
	}

	// 7. set start height and end height and save it to db
	btcDel.StartHeight = timeInfo.StartHeight
	btcDel.EndHeight = timeInfo.EndHeight
	k.setBTCDelegation(ctx, btcDel)

	// 8. emit events
	newInclusionProofEvent := types.NewInclusionProofEvent(
		stakingTxHash.String(),
		btcDel.StartHeight,
		btcDel.EndHeight,
		types.BTCDelegationStatus_ACTIVE,
	)

	if err := ctx.EventManager().EmitTypedEvents(newInclusionProofEvent); err != nil {
		panic(fmt.Errorf("failed to emit events for the new active BTC delegation: %w", err))
	}

	activeEvent := types.NewEventPowerDistUpdateWithBTCDel(
		&types.EventBTCDelegationStateUpdate{
			StakingTxHash: stakingTxHash.String(),
			NewState:      types.BTCDelegationStatus_ACTIVE,
		},
	)

	k.addPowerDistUpdateEvent(ctx, timeInfo.TipHeight, activeEvent)

	// record event that the BTC delegation will become unbonded at EndHeight-w
	expiredEvent := types.NewEventPowerDistUpdateWithBTCDel(&types.EventBTCDelegationStateUpdate{
		StakingTxHash: stakingTxHashStr,
		NewState:      types.BTCDelegationStatus_EXPIRED,
	})

	// NOTE: we should have verified that EndHeight > btcTip.Height + min_unbonding_time
	k.addPowerDistUpdateEvent(ctx, btcDel.EndHeight-params.UnbondingTimeBlocks, expiredEvent)

	return nil
}
