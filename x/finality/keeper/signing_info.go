package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
	"github.com/bits-and-blooms/bitset"

	bbntypes "github.com/babylonchain/babylon/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
)

// GetMissedBlockBitmapValue returns true if a finality provider missed signing
// a block at the given index and false otherwise. The index provided is assumed
// to be the index in the range [0, SignedBlocksWindow), which represents the bitmap
// where each bit represents a height, and is determined by the finality provider's
// IndexOffset modulo SignedBlocksWindow. This index is used to fetch the chunk
// in the bitmap and the relative bit in that chunk.
func (k Keeper) GetMissedBlockBitmapValue(ctx context.Context, fpPk *bbntypes.BIP340PubKey, index int64) (bool, error) {
	// get the chunk or "word" in the logical bitmap
	chunkIndex := index / finalitytypes.MissedBlockBitmapChunkSize

	bs := bitset.New(uint(finalitytypes.MissedBlockBitmapChunkSize))
	chunk, err := k.getMissedBlockBitmapChunk(ctx, fpPk, chunkIndex)
	if err != nil {
		return false, errorsmod.Wrapf(err, "failed to get bitmap chunk; index: %d", index)
	}

	if chunk != nil {
		if err := bs.UnmarshalBinary(chunk); err != nil {
			return false, errorsmod.Wrapf(err, "failed to decode bitmap chunk; index: %d", index)
		}
	}

	// get the bit position in the chunk of the logical bitmap, where Test()
	// checks if the bit is set.
	bitIndex := index % finalitytypes.MissedBlockBitmapChunkSize
	return bs.Test(uint(bitIndex)), nil
}

// SetMissedBlockBitmapValue sets, i.e. flips, a bit in the finality provider's missed
// block bitmap. When missed=true, the bit is set, otherwise it set to zero. The
// index provided is assumed to be the index in the range [0, SignedBlocksWindow),
// which represents the bitmap where each bit represents a height, and is
// determined by the finality provider's IndexOffset modulo SignedBlocksWindow. This
// index is used to fetch the chunk in the bitmap and the relative bit in that
// chunk.
func (k Keeper) SetMissedBlockBitmapValue(ctx context.Context, fpPk *bbntypes.BIP340PubKey, index int64, missed bool) error {
	// get the chunk or "word" in the logical bitmap
	chunkIndex := index / finalitytypes.MissedBlockBitmapChunkSize

	bs := bitset.New(uint(finalitytypes.MissedBlockBitmapChunkSize))
	chunk, err := k.getMissedBlockBitmapChunk(ctx, fpPk, chunkIndex)
	if err != nil {
		return errorsmod.Wrapf(err, "failed to get bitmap chunk; index: %d", index)
	}

	if chunk != nil {
		if err := bs.UnmarshalBinary(chunk); err != nil {
			return errorsmod.Wrapf(err, "failed to decode bitmap chunk; index: %d", index)
		}
	}

	// get the bit position in the chunk of the logical bitmap
	bitIndex := uint(index % finalitytypes.MissedBlockBitmapChunkSize)
	if missed {
		bs.Set(bitIndex)
	} else {
		bs.Clear(bitIndex)
	}

	updatedChunk, err := bs.MarshalBinary()
	if err != nil {
		return errorsmod.Wrapf(err, "failed to encode bitmap chunk; index: %d", index)
	}

	return k.SetMissedBlockBitmapChunk(ctx, fpPk, chunkIndex, updatedChunk)
}

// GetFinalityProviderMissedBlocks returns array of missed blocks for given finality provider.
// Adapted from
// https://github.com/cosmos/cosmos-sdk/blob/f499bbf2138b171d6e5396a37df7699952e76bf3/x/slashing/keeper/signing_info.go#L224
func (k Keeper) GetFinalityProviderMissedBlocks(ctx context.Context, fpPk *bbntypes.BIP340PubKey) ([]finalitytypes.MissedBlock, error) {
	signedBlocksWindow := k.GetParams(ctx).SignedBlocksWindow

	missedBlocks := make([]finalitytypes.MissedBlock, 0, signedBlocksWindow)
	err := k.IterateMissedBlockBitmap(ctx, fpPk, func(index int64, missed bool) (stop bool) {
		if missed {
			missedBlocks = append(missedBlocks, finalitytypes.MissedBlock{Index: index, Missed: missed})
		}

		return false
	})

	return missedBlocks, err
}

// IterateMissedBlockBitmap iterates over a finality provider's signed blocks window
// bitmap and performs a callback function on each index, i.e. block height, in
// the range [0, SignedBlocksWindow).
// Note: A callback will only be executed over all bitmap chunks that exist in
// state.
// Adapted from
// https://github.com/cosmos/cosmos-sdk/blob/f499bbf2138b171d6e5396a37df7699952e76bf3/x/slashing/keeper/signing_info.go#L202
func (k Keeper) IterateMissedBlockBitmap(ctx context.Context, fpPk *bbntypes.BIP340PubKey, cb func(index int64, missed bool) (stop bool)) error {
	var index int64
	rng := collections.NewPrefixedPairRange[[]byte, uint64](fpPk.MustMarshal())
	return k.FinalityProviderMissedBlockBitmap.Walk(ctx, rng, func(key collections.Pair[[]byte, uint64], value []byte) (bool, error) {
		bs := bitset.New(uint(finalitytypes.MissedBlockBitmapChunkSize))

		if err := bs.UnmarshalBinary(value); err != nil {
			return true, errorsmod.Wrapf(err, "failed to decode bitmap chunk; index: %v", key)
		}

		for i := uint(0); i < finalitytypes.MissedBlockBitmapChunkSize; i++ {
			// execute the callback, where Test() returns true if the bit is set
			if cb(index, bs.Test(i)) {
				break
			}

			index++
		}
		return false, nil
	})
}

// DeleteMissedBlockBitmap removes a finality provider's missed block bitmap from state.
func (k Keeper) DeleteMissedBlockBitmap(ctx context.Context, fpPk *bbntypes.BIP340PubKey) error {
	rng := collections.NewPrefixedPairRange[[]byte, uint64](fpPk.MustMarshal())
	return k.FinalityProviderMissedBlockBitmap.Clear(ctx, rng)
}

// SetMissedBlockBitmapChunk sets the bitmap chunk at the given chunk index for
// a finality provider's missed block signing window.
func (k Keeper) SetMissedBlockBitmapChunk(ctx context.Context, fpPk *bbntypes.BIP340PubKey, chunkIndex int64, chunk []byte) error {
	return k.FinalityProviderMissedBlockBitmap.Set(ctx, collections.Join(fpPk.MustMarshal(), uint64(chunkIndex)), chunk)
}

// getMissedBlockBitmapChunk gets the bitmap chunk at the given chunk index for
// a finality provider's missed block signing window.
func (k Keeper) getMissedBlockBitmapChunk(ctx context.Context, fpPk *bbntypes.BIP340PubKey, chunkIndex int64) ([]byte, error) {
	chunk, err := k.FinalityProviderMissedBlockBitmap.Get(ctx, collections.Join(fpPk.MustMarshal(), uint64(chunkIndex)))
	if err != nil && !errors.Is(err, collections.ErrNotFound) {
		return nil, err
	}
	return chunk, nil
}
