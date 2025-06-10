package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
)

func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	// set validator sets per epoch
	valSetStore := k.valBlsSetStore(ctx)
	for _, e := range gs.ValidatorSets {
		valBlsSetBytes := types.ValidatorBlsKeySetToBytes(k.cdc, e.ValidatorSet)
		valSetStore.Set(types.ValidatorBlsKeySetKey(e.EpochNumber), valBlsSetBytes)
	}

	// set epochs checkpoints
	cs := k.CheckpointsState(ctx)
	for _, c := range gs.Checkpoints {
		if err := cs.CreateRawCkptWithMeta(c); err != nil {
			return err
		}
	}

	// set last finalized epoch
	k.SetLastFinalizedEpoch(ctx, gs.LastFinalizedEpoch)

	// set genesis BLS keys
	return k.SetGenBlsKeys(ctx, gs.GenesisKeys)
}

// ExportGenesis returns the keeper state into a exported genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	// Note that validator ed25519 pub key and PoP are not stored in the module
	// but used on InitGenesis for validation. Make sure to populate these fields
	// before using the exported data as input in the InitGenesis logic
	gs, err := k.GetBlsKeys(ctx)
	if err != nil {
		return nil, err
	}

	vs, err := k.validatorSets(ctx)
	if err != nil {
		return nil, err
	}

	cs, err := k.checkpoints(ctx)
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		GenesisKeys:        gs,
		ValidatorSets:      vs,
		Checkpoints:        cs,
		LastFinalizedEpoch: k.GetLastFinalizedEpoch(ctx),
	}, nil
}

func (k Keeper) validatorSets(ctx context.Context) ([]*types.ValidatorSetEntry, error) {
	entries := make([]*types.ValidatorSetEntry, 0)
	store := k.valBlsSetStore(ctx)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		epochNum := sdk.BigEndianToUint64(iter.Key())

		vSet := new(types.ValidatorWithBlsKeySet)
		if err := vSet.Unmarshal(iter.Value()); err != nil {
			return nil, err
		}

		entries = append(entries, &types.ValidatorSetEntry{
			EpochNumber:  epochNum,
			ValidatorSet: vSet,
		})
	}

	return entries, nil
}

func (k Keeper) checkpoints(ctx context.Context) ([]*types.RawCheckpointWithMeta, error) {
	cs := make([]*types.RawCheckpointWithMeta, 0)
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, types.CkptsObjectPrefix)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		chkpt := new(types.RawCheckpointWithMeta)
		if err := chkpt.Unmarshal(iter.Value()); err != nil {
			return nil, err
		}
		cs = append(cs, chkpt)
	}
	return cs, nil
}
