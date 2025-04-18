package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis performs stateful validations and initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	k.setLastFinalizedEpochNumber(ctx, gs.LastFinalizedEpochNumber)

	for _, e := range gs.Epochs {
		k.saveEpochData(ctx, e.EpochNumber, e.Data)
	}

	for _, s := range gs.Submissions {
		k.saveSubmission(ctx, *s.SubmissionKey, *s.Data)
	}

	return k.SetParams(ctx, gs.Params)
}

// ExportGenesis returns the keeper state into a exported genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	e, err := k.epochs(ctx)
	if err != nil {
		return nil, err
	}

	s, err := k.submissions(ctx)
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		Params:                   k.GetParams(ctx),
		LastFinalizedEpochNumber: k.getLastFinalizedEpochNumber(ctx),
		Epochs:                   e,
		Submissions:              s,
	}, nil
}

// epochs loads all epochs data stored.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) epochs(ctx context.Context) ([]types.EpochEntry, error) {
	entries := make([]types.EpochEntry, 0)

	iter := k.epochDataStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var epoch types.EpochData
		if err := k.cdc.Unmarshal(iter.Value(), &epoch); err != nil {
			return nil, err
		}
		epochNum := sdk.BigEndianToUint64(iter.Key())
		entry := types.EpochEntry{
			EpochNumber: epochNum,
			Data:        &epoch,
		}
		if err := entry.Validate(); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// epochs loads all submissions data stored.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) submissions(ctx context.Context) ([]types.SubmissionEntry, error) {
	entries := make([]types.SubmissionEntry, 0)
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	submissionsDataStore := prefix.NewStore(storeAdapter, types.SubmisionKeyPrefix)

	iter := submissionsDataStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var data types.SubmissionData
		if err := k.cdc.Unmarshal(iter.Value(), &data); err != nil {
			return nil, err
		}
		var sk types.SubmissionKey
		if err := k.cdc.Unmarshal(iter.Key(), &sk); err != nil {
			return nil, err
		}
		entry := types.SubmissionEntry{
			SubmissionKey: &sk,
			Data:          &data,
		}
		if err := entry.Validate(); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
