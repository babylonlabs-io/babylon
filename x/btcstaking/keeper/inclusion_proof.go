package keeper

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	sdk "github.com/cosmos/cosmos-sdk/types"

	btcckpttypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

// VerifyInclusionProofAndGetHeight verifies the inclusion proof of the given staking tx
// and returns the inclusion height
func (k Keeper) VerifyInclusionProofAndGetHeight(
	ctx sdk.Context,
	stakingTx *btcutil.Tx,
	stakingTime uint64,
	inclusionProof *types.ParsedProofOfInclusion,
) (uint64, error) {
	btccParams := k.btccKeeper.GetParams(ctx)
	// Check:
	// - timelock of staking tx
	// - staking tx is k-deep
	// - staking tx inclusion proof
	stakingTxHeader := k.btclcKeeper.GetHeaderByHash(ctx, inclusionProof.HeaderHash)

	if stakingTxHeader == nil {
		return 0, fmt.Errorf("header that includes the staking tx is not found")
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
		return 0, types.ErrInvalidStakingTx.Wrapf("not included in the Bitcoin chain")
	}

	startHeight := stakingTxHeader.Height
	endHeight := stakingTxHeader.Height + stakingTime

	btcTip := k.btclcKeeper.GetTipInfo(ctx)
	stakingTxDepth := btcTip.Height - stakingTxHeader.Height
	if stakingTxDepth < btccParams.BtcConfirmationDepth {
		return 0, types.ErrInvalidStakingTx.Wrapf("not k-deep: k=%d; depth=%d", btccParams.BtcConfirmationDepth, stakingTxDepth)
	}
	// ensure staking tx's timelock has more than w BTC blocks left
	if btcTip.Height+btccParams.CheckpointFinalizationTimeout >= endHeight {
		return 0, types.ErrInvalidStakingTx.Wrapf("staking tx's timelock has no more than w(=%d) blocks left", btccParams.CheckpointFinalizationTimeout)
	}

	return startHeight, nil
}
