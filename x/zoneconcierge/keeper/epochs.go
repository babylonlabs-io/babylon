package keeper

import (
	"context"

	epochingtypes "github.com/babylonlabs-io/babylon/v3/x/epoching/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

// GetLastSentSegment get last broadcasted btc light client segment
func (k Keeper) GetLastSentSegment(ctx context.Context) *types.BTCChainSegment {
	segment, err := k.LastSentBTCSegment.Get(ctx)
	if err != nil {
		return nil
	}
	return &segment
}

// setLastSentSegment sets the last segment which was broadcasted to the other light clients
// called upon each AfterRawCheckpointFinalized hook invocation
func (k Keeper) setLastSentSegment(ctx context.Context, segment *types.BTCChainSegment) error {
	return k.LastSentBTCSegment.Set(ctx, *segment)
}

func (k Keeper) GetLastFinalizedEpoch(ctx context.Context) uint64 {
	return k.checkpointingKeeper.GetLastFinalizedEpoch(ctx)
}

func (k Keeper) GetEpoch(ctx context.Context) *epochingtypes.Epoch {
	return k.epochingKeeper.GetEpoch(ctx)
}

func (k Keeper) recordSealedEpochProof(ctx context.Context, epochNum uint64) {
	// proof that the epoch is sealed
	proofEpochSealed, err := k.ProveEpochSealed(ctx, epochNum)
	if err != nil {
		panic(err) // only programming error
	}

	if err := k.SealedEpochProof.Set(ctx, epochNum, *proofEpochSealed); err != nil {
		panic(err)
	}
}

func (k Keeper) getSealedEpochProof(ctx context.Context, epochNum uint64) *types.ProofEpochSealed {
	proof, err := k.SealedEpochProof.Get(ctx, epochNum)
	if err != nil {
		return nil
	}
	return &proof
}
