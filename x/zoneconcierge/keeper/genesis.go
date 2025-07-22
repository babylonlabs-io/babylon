package keeper

import (
	"context"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	k.SetPort(ctx, gs.PortId)

	// Initialize finalized headers
	for _, fh := range gs.FinalizedHeaders {
		k.setFinalizedHeader(ctx, fh.ConsumerId, fh.EpochNumber, fh.HeaderWithProof)
	}

	// Initialize last sent segment
	if gs.LastSentSegment != nil {
		k.setLastSentSegment(ctx, gs.LastSentSegment)
	}

	// Initialize sealed epoch proofs
	for _, se := range gs.SealedEpochsProofs {
		k.sealedEpochProofStore(ctx).Set(
			sdk.Uint64ToBigEndian(se.EpochNumber),
			k.cdc.MustMarshal(se.Proof),
		)
	}

	// Initialize consumer BTC states
	for _, cs := range gs.BsnBtcStates {
		k.SetBSNBTCState(ctx, cs.ConsumerId, cs.State)
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
		PortId:             k.GetPort(ctx),
		FinalizedHeaders:   fh,
		LastSentSegment:    k.GetLastSentSegment(ctx),
		SealedEpochsProofs: se,
		BsnBtcStates:       cs,
	}, nil
}

// getAllBSNBTCStates gets all BSN BTC states for genesis export
func (k Keeper) getAllBSNBTCStates(ctx context.Context) []*types.BSNBTCStateEntry {
	var entries []*types.BSNBTCStateEntry
	store := k.bsnBTCStateStore(ctx)
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		consumerID := string(iterator.Key())
		var state types.BSNBTCState
		k.cdc.MustUnmarshal(iterator.Value(), &state)
		entries = append(entries, &types.BSNBTCStateEntry{
			ConsumerId: consumerID,
			State:      &state,
		})
	}
	return entries
}

// sealedEpochsProofs gets all sealed epoch proofs for genesis export
func (k Keeper) sealedEpochsProofs(ctx context.Context) ([]*types.SealedEpochProofEntry, error) {
	entries := make([]*types.SealedEpochProofEntry, 0)
	iter := k.sealedEpochProofStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		epochNum := sdk.BigEndianToUint64(iter.Key())

		var proof types.ProofEpochSealed
		if err := k.cdc.Unmarshal(iter.Value(), &proof); err != nil {
			return nil, err
		}
		entry := &types.SealedEpochProofEntry{
			EpochNumber: epochNum,
			Proof:       &proof,
		}
		if err := entry.Validate(); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
