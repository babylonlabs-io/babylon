package keeper

import (
	"context"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"math"

	"cosmossdk.io/store/prefix"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
)

func (k Keeper) HasSubmission(ctx context.Context, sk types.SubmissionKey) bool {
	store := k.storeService.OpenKVStore(ctx)
	kBytes := types.PrefixedSubmisionKey(k.cdc, &sk)
	has, err := store.Has(kBytes)
	if err != nil {
		panic(err)
	}
	return has
}

// GetBestSubmission gets the status and the best submission of a given finalized epoch
func (k Keeper) GetBestSubmission(ctx context.Context, epochNumber uint64) (types.BtcStatus, *types.SubmissionKey, error) {
	// find the btc checkpoint tx index of this epoch
	ed := k.GetEpochData(ctx, epochNumber)
	if ed == nil {
		return 0, nil, types.ErrNoCheckpointsForPreviousEpoch
	}
	if ed.Status != types.Finalized {
		return 0, nil, fmt.Errorf("epoch %d has not been finalized yet", epochNumber)
	}
	if len(ed.Keys) == 0 {
		return 0, nil, types.ErrNoCheckpointsForPreviousEpoch
	}
	bestSubmissionKey := ed.Keys[0] // index of checkpoint tx on BTC

	return ed.Status, bestSubmissionKey, nil
}

// addEpochSubmission save given submission key and data to database and takes
// car of updating any necessary indexes.
// Provided submmission should be known to btclightclient and all of its blocks
// should be on btc main chaing as viewed by btclightclient
func (k Keeper) addEpochSubmission(
	ctx context.Context,
	epochNum uint64,
	sk types.SubmissionKey,
	sd types.SubmissionData,
) error {

	ed := k.GetEpochData(ctx, epochNum)

	// TODO: SaveEpochData and SaveSubmission should be done in one transaction.
	// Not sure cosmos-sdk has facilities to do it.
	// Otherwise it is possible to end up with node which updated submission list
	// but did not save submission itself.

	// if ed is nil, it means it is our first submission for this epoch
	if ed == nil {
		// we do not have any data saved yet
		newEd := types.NewEmptyEpochData()
		ed = &newEd
	}

	if ed.Status == types.Finalized {
		// we already finalized given epoch so we do not need any more submissions
		// TODO We should probably compare new submission with the existing submission
		// which finalized the epoch. As it means we finalized epoch with not the best
		// submission possible
		return types.ErrEpochAlreadyFinalized
	}

	if len(ed.Keys) == 0 {
		// it is first epoch submission inform checkpointing module about this fact
		k.checkpointingKeeper.SetCheckpointSubmitted(ctx, epochNum)
	}

	ed.AppendKey(sk)
	k.saveEpochData(ctx, epochNum, ed)
	k.saveSubmission(ctx, sk, sd)
	return nil
}

func (k Keeper) saveSubmission(ctx context.Context, sk types.SubmissionKey, sd types.SubmissionData) {
	store := k.storeService.OpenKVStore(ctx)
	kBytes := types.PrefixedSubmisionKey(k.cdc, &sk)
	sBytes := k.cdc.MustMarshal(&sd)
	if err := store.Set(kBytes, sBytes); err != nil {
		panic(err)
	}
}

func (k Keeper) deleteSubmission(ctx context.Context, sk types.SubmissionKey) {
	store := k.storeService.OpenKVStore(ctx)
	kBytes := types.PrefixedSubmisionKey(k.cdc, &sk)
	if err := store.Delete(kBytes); err != nil {
		panic(err)
	}
}

// GetSubmissionData returns submission data for a given key or nil if there is no data
// under the given key
func (k Keeper) GetSubmissionData(ctx context.Context, sk types.SubmissionKey) *types.SubmissionData {
	store := k.storeService.OpenKVStore(ctx)
	kBytes := types.PrefixedSubmisionKey(k.cdc, &sk)
	sdBytes, err := store.Get(kBytes)
	if err != nil {
		panic(err)
	}

	if len(sdBytes) == 0 {
		return nil
	}

	var sd types.SubmissionData
	k.cdc.MustUnmarshal(sdBytes, &sd)
	return &sd
}

func (k Keeper) checkSubmissionStatus(ctx context.Context, info *types.SubmissionBtcInfo) types.BtcStatus {
	subDepth := info.SubmissionDepth()
	if subDepth >= k.GetParams(ctx).CheckpointFinalizationTimeout {
		return types.Finalized
	} else if subDepth >= k.GetParams(ctx).BtcConfirmationDepth {
		return types.Confirmed
	} else {
		return types.Submitted
	}
}

func (k Keeper) GetSubmissionBtcInfo(ctx context.Context, sk types.SubmissionKey) (*types.SubmissionBtcInfo, error) {

	var youngestBlockDepth uint64 = math.MaxUint64
	var youngestBlockHash *bbn.BTCHeaderHashBytes

	var lowestIndexInMostFreshBlock uint32 = math.MaxUint32

	var oldestBlockDepth = uint64(0)

	for _, tk := range sk.Key {
		currentBlockDepth, err := k.headerDepth(ctx, tk.Hash)

		if err != nil {
			return nil, err
		}

		if currentBlockDepth < youngestBlockDepth {
			youngestBlockDepth = currentBlockDepth
			lowestIndexInMostFreshBlock = tk.Index
			youngestBlockHash = tk.Hash
		}

		// This case happens when we have two submissions in the same block.
		if currentBlockDepth == youngestBlockDepth && tk.Index < lowestIndexInMostFreshBlock {
			// This is something which needs a bit more careful thinking as it is used
			// to determine which submission is better.
			// Currently if two submissions of one checkpoint are in the same block,
			// we pick tx with lower index as the point at which checkpoint happened.
			// This is in line with the logic that if two submission are in the same block,
			// they are essentially happening at the same time, so it does not really matter
			// which index pick, and for possible tie breaks it is better to pick lower one.
			// This means in case when we have:
			// Checkpoint submission `x` for epoch 5, both tx in same block at height 100, with indexes 1 and 10
			// and
			// Checkpoint submission `y` for epoch 5, both tx in same block at height 100, with indexes 3 and 9
			// we will chose submission `x` as the better one.
			// This good enough solution, but it is not perfect and leads to some edge cases like:
			// Checkpoint submission `x` for epoch 5, one tx in block 99 with index 1, and second tx in block 100 with index 4
			// and
			// Checkpoint submission `y` for epoch 5, both tx in same block at height 100, with indexes 3 and 9
			// In this case submission `y` will be better as it `earliest` tx in most fresh block is first. But at first glance
			// submission `x` seems better.
			lowestIndexInMostFreshBlock = tk.Index
		}

		if currentBlockDepth > oldestBlockDepth {
			oldestBlockDepth = currentBlockDepth
		}
	}

	return &types.SubmissionBtcInfo{
		SubmissionKey:            sk,
		OldestBlockDepth:         oldestBlockDepth,
		YoungestBlockDepth:       youngestBlockDepth,
		YoungestBlockHash:        *youngestBlockHash,
		YoungestBlockLowestTxIdx: lowestIndexInMostFreshBlock,
	}, nil
}

func (k Keeper) GetEpochBestSubmissionBtcInfo(ctx context.Context, ed *types.EpochData) *types.SubmissionBtcInfo {
	// there are no submissions for this epoch, so transitively there is no best submission
	if ed == nil || len(ed.Keys) == 0 {
		return nil
	}

	// There is only one submission for this epoch:
	// - either epoch is already finalized, and we already chose the best submission
	// - or we only received one submission for this epoch
	// Either way, we do not need to decide which submission is the best one.
	if len(ed.Keys) == 1 {
		sk := *ed.Keys[0]
		btcInfo, err := k.GetSubmissionBtcInfo(ctx, sk)

		if err != nil {
			k.Logger(sdk.UnwrapSDKContext(ctx)).Debug("Previously stored submission is not valid anymore. Submission key: %+v", sk)
		}

		// we only log error, as the only error which we can receive here is that submission
		// is no longer on btc canonical chain, which essentially means that there is no valid submission
		return btcInfo
	}

	// We have more that one valid submission. We need to chose the best one.
	epochSummary := k.getEpochChanges(ctx, nil, ed)

	return epochSummary.EpochBestSubmission
}

// GetEpochData returns epoch data for given epoch, if there is not epoch data yet returns nil
func (k Keeper) GetEpochData(ctx context.Context, e uint64) *types.EpochData {
	store := k.storeService.OpenKVStore(ctx)
	bytes, err := store.Get(types.GetEpochIndexKey(e))
	if err != nil {
		panic(err)
	}

	// note: Cannot check len(bytes) == 0, as empty bytes encoding of types.EpochData
	// is epoch data with Status == Submitted and no valid submissions
	if bytes == nil {
		return nil
	}

	ed := &types.EpochData{}
	k.cdc.MustUnmarshal(bytes, ed)
	return ed
}

func (k Keeper) saveEpochData(ctx context.Context, e uint64, ed *types.EpochData) {
	store := k.storeService.OpenKVStore(ctx)
	ek := types.GetEpochIndexKey(e)
	eb := k.cdc.MustMarshal(ed)
	if err := store.Set(ek, eb); err != nil {
		panic(err)
	}
}

func (k Keeper) clearEpochData(
	ctx context.Context,
	epoch []byte,
	epochDataStore prefix.Store,
	currentEpoch *types.EpochData) {
	for _, sk := range currentEpoch.Keys {
		k.deleteSubmission(ctx, *sk)
	}
	currentEpoch.Keys = []*types.SubmissionKey{}
	epochDataStore.Set(epoch, k.cdc.MustMarshal(currentEpoch))
}
