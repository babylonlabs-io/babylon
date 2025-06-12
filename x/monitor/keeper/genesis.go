package keeper

import (
	"context"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	chkpttypes "github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
	"github.com/babylonlabs-io/babylon/v3/x/monitor/types"
)

// InitGenesis initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	store := k.storeService.OpenKVStore(ctx)
	for _, e := range gs.EpochEndRecords {
		k := types.GetEpochEndLightClientHeightKey(e.Epoch)
		v := sdk.Uint64ToBigEndian(e.BtcLightClientHeight)
		if err := store.Set(k, v); err != nil {
			return err
		}
	}
	for _, c := range gs.CheckpointsReported {
		k, err := types.GetCheckpointReportedLightClientHeightKey(c.CkptHash)
		if err != nil {
			return err
		}
		v := sdk.Uint64ToBigEndian(c.BtcLightClientHeight)
		if err := store.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

// ExportGenesis returns the keeper state into a exported genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	es, err := k.epochEndRecords(ctx)
	if err != nil {
		return nil, err
	}

	cs, err := k.checkpointsReported(ctx)
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		EpochEndRecords:     es,
		CheckpointsReported: cs,
	}, nil
}

func (k Keeper) epochEndRecords(ctx context.Context) ([]*types.EpochEndLightClient, error) {
	var (
		records      = make([]*types.EpochEndLightClient, 0)
		storeAdapter = runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
		store        = prefix.NewStore(storeAdapter, types.EpochEndLightClientHeightPrefix)
		iter         = store.Iterator(nil, nil)
	)

	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		record := &types.EpochEndLightClient{
			Epoch:                sdk.BigEndianToUint64(iter.Key()),
			BtcLightClientHeight: sdk.BigEndianToUint64(iter.Value()),
		}
		records = append(records, record)
	}
	return records, nil
}

func (k Keeper) checkpointsReported(ctx context.Context) ([]*types.CheckpointReportedLightClient, error) {
	var (
		ckpts        = make([]*types.CheckpointReportedLightClient, 0)
		storeAdapter = runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
		store        = prefix.NewStore(storeAdapter, types.CheckpointReportedLightClientHeightPrefix)
		iter         = store.Iterator(nil, nil)
	)

	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		hash := chkpttypes.RawCkptHash(iter.Key())
		ckpt := &types.CheckpointReportedLightClient{
			CkptHash:             hash.String(),
			BtcLightClientHeight: sdk.BigEndianToUint64(iter.Value()),
		}
		if err := ckpt.Validate(); err != nil {
			return nil, err
		}
		ckpts = append(ckpts, ckpt)
	}
	return ckpts, nil
}
