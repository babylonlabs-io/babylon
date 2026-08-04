package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

// HandleRewarding calls the reward to stakers if the block is finalized
func (k Keeper) HandleRewarding(ctx context.Context, targetHeight int64, maxRewardedBlocks uint64) {
	// rewarding is executed in a range of [nextHeightToReward, heightToExamine]
	// this is we don't know when a block will be finalized and we need ensure
	// every finalized block will be processed to reward
	nextHeightToReward := k.GetNextHeightToReward(ctx)
	if nextHeightToReward == 0 {
		// first time to call reward, set it to activated height
		activatedHeight, err := k.GetBTCStakingActivatedHeight(ctx)
		if err != nil {
			panic(err)
		}
		nextHeightToReward = activatedHeight
	}

	maxHeightToReward := min(
		// need to add minus 1, as the rewarding loop is inclucive of [start, end]
		nextHeightToReward+maxRewardedBlocks-1,
		uint64(targetHeight),
	)

	copiedNextHeightToReward := nextHeightToReward

	for height := nextHeightToReward; height <= maxHeightToReward; height++ {
		block, err := k.GetBlock(ctx, height)
		if err != nil {
			panic(err)
		}
		if !block.Finalized {
			continue
		}
		k.rewardBTCStaking(ctx, height)
		nextHeightToReward = height + 1
	}

	if nextHeightToReward != copiedNextHeightToReward {
		k.SetNextHeightToReward(ctx, nextHeightToReward)
	}
}

func (k Keeper) rewardBTCStaking(ctx context.Context, height uint64) {
	// distribute rewards to BTC staking stakeholders w.r.t. the voting power distribution cache
	dc := k.GetVotingPowerDistCache(ctx, height)
	if dc == nil {
		// failing to get a voting power distribution cache before distributing reward is a programming error
		panic(fmt.Errorf("voting power distribution cache not found at height %d", height))
	}

	// get all the voters for the height
	voterBTCPKs := k.GetVoters(ctx, height)

	// reward active finality providers
	k.IncentiveKeeper.RewardBTCStaking(ctx, height, dc, voterBTCPKs)

	// remove reward distribution cache afterwards
	k.RemoveVotingPowerDistCache(ctx, height)
}

// SetNextHeightToReward sets the next height to reward as the given height
func (k Keeper) SetNextHeightToReward(ctx context.Context, height uint64) {
	store := k.storeService.OpenKVStore(ctx)
	heightBytes := sdk.Uint64ToBigEndian(height)
	if err := store.Set(types.NextHeightToRewardKey, heightBytes); err != nil {
		panic(err)
	}
}

// GetNextHeightToReward gets the next height to reward
func (k Keeper) GetNextHeightToReward(ctx context.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.NextHeightToRewardKey)
	if err != nil {
		panic(err)
	}
	if bz == nil {
		return 0
	}
	height := sdk.BigEndianToUint64(bz)
	return height
}
