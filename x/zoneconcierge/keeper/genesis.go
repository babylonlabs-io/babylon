package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
)

// InitGenesis initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	// Initialize finalized headers
	for _, fh := range gs.FinalizedHeaders {
		k.setFinalizedHeader(ctx, fh.ConsumerId, fh.EpochNumber, fh.HeaderWithProof)
	}

	// Initialize sealed epoch proofs
	for _, se := range gs.SealedEpochsProofs {
		if err := k.SealedEpochProof.Set(ctx, se.EpochNumber, *se.Proof); err != nil {
			return err
		}
	}

	// Initialize consumer BTC states
	for _, cs := range gs.BsnBtcStates {
		if err := k.SetBSNBTCState(ctx, cs.ConsumerId, cs.State); err != nil {
			return err
		}
	}

	// NOTE: Consumer registration is now handled by the btcstkconsumer module

	return k.SetParams(ctx, gs.Params)
}

// ExportGenesis returns the keeper state into a exported genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	// Get all finalized headers
	fh := k.GetAllFinalizedHeaders(ctx)

	// Get all consumer BTC states
	cs := k.getAllBSNBTCStates(ctx)

	// Get sealed epoch proofs
	se, err := k.sealedEpochsProofs(ctx)
	if err != nil {
		return nil, err
	}

	// NOTE: Consumer registration is now handled by the btcstkconsumer module

	return &types.GenesisState{
		Params:             k.GetParams(ctx),
		FinalizedHeaders:   fh,
		SealedEpochsProofs: se,
		BsnBtcStates:       cs,
	}, nil
}

// getAllBSNBTCStates gets all BSN BTC states for genesis export
func (k Keeper) getAllBSNBTCStates(ctx context.Context) []*types.BSNBTCStateEntry {
	var entries []*types.BSNBTCStateEntry
	err := k.BSNBTCState.Walk(ctx, nil, func(consumerID string, state types.BSNBTCState) (bool, error) {
		entries = append(entries, &types.BSNBTCStateEntry{
			ConsumerId: consumerID,
			State:      &state,
		})
		return false, nil
	})
	if err != nil {
		panic(err)
	}
	return entries
}

// sealedEpochsProofs gets all sealed epoch proofs for genesis export
func (k Keeper) sealedEpochsProofs(ctx context.Context) ([]*types.SealedEpochProofEntry, error) {
	entries := make([]*types.SealedEpochProofEntry, 0)
	err := k.SealedEpochProof.Walk(ctx, nil, func(epochNum uint64, proof types.ProofEpochSealed) (bool, error) {
		entry := &types.SealedEpochProofEntry{
			EpochNumber: epochNum,
			Proof:       &proof,
		}
		if err := entry.Validate(); err != nil {
			return true, err
		}
		entries = append(entries, entry)
		return false, nil
	})
	return entries, err
}
