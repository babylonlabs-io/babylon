package keeper

import (
	"context"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/babylonlabs-io/babylon/x/checkpointing/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type CheckpointsState struct {
	cdc         codec.BinaryCodec
	checkpoints storetypes.KVStore
}

func (k Keeper) CheckpointsState(ctx context.Context) CheckpointsState {
	// Build the CheckpointsState storage
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return CheckpointsState{
		cdc:         k.cdc,
		checkpoints: prefix.NewStore(storeAdapter, types.CkptsObjectPrefix),
	}
}

// CreateRawCkptWithMeta inserts the raw checkpoint with meta into the storage by its epoch number
// a new checkpoint is created with the status of UNCEHCKPOINTED
func (cs CheckpointsState) CreateRawCkptWithMeta(ckptWithMeta *types.RawCheckpointWithMeta) error {
	if ckptWithMeta == nil {
		return types.ErrInvalidRawCheckpoint.Wrapf("empty raw checkpoint")
	}
	epoch := ckptWithMeta.Ckpt.EpochNum
	if cs.checkpoints.Has(types.CkptsObjectKey(epoch)) {
		return types.ErrCkptAlreadyExist.Wrapf("a raw checkpoint already exists at epoch %v", epoch)
	}

	// save concrete ckpt object
	cs.checkpoints.Set(types.CkptsObjectKey(epoch), types.CkptWithMetaToBytes(cs.cdc, ckptWithMeta))
	return nil
}

// GetRawCkptWithMeta retrieves a raw checkpoint with meta by its epoch number
func (cs CheckpointsState) GetRawCkptWithMeta(epoch uint64) (*types.RawCheckpointWithMeta, error) {
	ckptsKey := types.CkptsObjectKey(epoch)
	rawBytes := cs.checkpoints.Get(ckptsKey)
	if rawBytes == nil {
		return nil, types.ErrCkptDoesNotExist.Wrapf("no raw checkpoint is found at epoch %v", epoch)
	}

	return types.BytesToCkptWithMeta(cs.cdc, rawBytes)
}

// GetRawCkptsWithMetaByStatus retrieves raw checkpoints with meta by their status by the descending order of epoch
func (cs CheckpointsState) GetRawCkptsWithMetaByStatus(status types.CheckpointStatus, f func(*types.RawCheckpointWithMeta) bool) error {
	store := prefix.NewStore(cs.checkpoints, types.CkptsObjectPrefix)
	iter := store.ReverseIterator(nil, nil)
	defer iter.Close()

	// the iterator starts from the highest epoch number
	// once it gets to an epoch where the status is CONFIRMED,
	// all the lower epochs will be CONFIRMED
	for ; iter.Valid(); iter.Next() {
		ckptBytes := iter.Value()
		ckptWithMeta, err := types.BytesToCkptWithMeta(cs.cdc, ckptBytes)
		if err != nil {
			return err
		}
		// the loop can end if the current status is CONFIRMED but the requested status is not CONFIRMED
		if status != types.Confirmed && ckptWithMeta.Status == types.Confirmed {
			return nil
		}
		if ckptWithMeta.Status != status {
			continue
		}
		stop := f(ckptWithMeta)
		if stop {
			return nil
		}
	}
	return nil
}

// UpdateCkptStatus updates the checkpoint's status
func (cs CheckpointsState) UpdateCkptStatus(ckpt *types.RawCheckpoint, status types.CheckpointStatus) error {
	ckptWithMeta, err := cs.GetRawCkptWithMeta(ckpt.EpochNum)
	if err != nil {
		// the checkpoint should exist
		return err
	}
	if !ckptWithMeta.Ckpt.Hash().Equals(ckpt.Hash()) {
		return types.ErrCkptHashNotEqual.Wrapf("conflicting hash at epoch %v", ckpt.EpochNum)
	}
	ckptWithMeta.Status = status
	cs.checkpoints.Set(sdk.Uint64ToBigEndian(ckpt.EpochNum), types.CkptWithMetaToBytes(cs.cdc, ckptWithMeta))

	return nil
}

// UpdateCheckpoint overwrites an existing checkpoint
func (cs CheckpointsState) UpdateCheckpoint(ckpt *types.RawCheckpointWithMeta) error {
	_, err := cs.GetRawCkptWithMeta(ckpt.Ckpt.EpochNum)
	if err != nil {
		// the checkpoint should exist
		return err
	}

	cs.checkpoints.Set(sdk.Uint64ToBigEndian(ckpt.Ckpt.EpochNum), types.CkptWithMetaToBytes(cs.cdc, ckpt))

	return nil
}
