package keeper

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	sdk "github.com/cosmos/cosmos-sdk/types"

	btcckpttypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
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
