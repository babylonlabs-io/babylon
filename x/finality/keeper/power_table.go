package keeper

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/runtime"

	"cosmossdk.io/store/prefix"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/finality/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) SetVotingPower(ctx context.Context, fpBTCPK []byte, height uint64, power uint64) {
	store := k.votingPowerBbnBlockHeightStore(ctx, height)
	store.Set(fpBTCPK, sdk.Uint64ToBigEndian(power))
}

// GetVotingPower gets the voting power of a given finality provider at a given Babylon height
func (k Keeper) GetVotingPower(ctx context.Context, fpBTCPK []byte, height uint64) uint64 {
	store := k.votingPowerBbnBlockHeightStore(ctx, height)
	powerBytes := store.Get(fpBTCPK)
	if len(powerBytes) == 0 {
		return 0
	}
	return sdk.BigEndianToUint64(powerBytes)
}

// GetCurrentVotingPower gets the voting power of a given finality provider at the current height
// NOTE: it's possible that the voting power table is 1 block behind CometBFT, e.g., when `BeginBlock`
// hasn't executed yet
func (k Keeper) GetCurrentVotingPower(ctx context.Context, fpBTCPK []byte) (uint64, uint64) {
	// find the last recorded voting power table via iterator
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, types.VotingPowerKey)
	iter := store.ReverseIterator(nil, nil)
	defer iter.Close()

	// no voting power table is known yet, return 0
	if !iter.Valid() {
		return 0, 0
	}

	// there is known voting power table, find the last height
	lastHeight := sdk.BigEndianToUint64(iter.Key())
	storeAtHeight := prefix.NewStore(store, sdk.Uint64ToBigEndian(lastHeight))

	// if the finality provider is not known, return 0 voting power
	if !k.BTCStakingKeeper.HasFinalityProvider(ctx, fpBTCPK) {
		return lastHeight, 0
	}

	// find the voting power of this finality provider
	powerBytes := storeAtHeight.Get(fpBTCPK)
	if len(powerBytes) == 0 {
		return lastHeight, 0
	}

	return lastHeight, sdk.BigEndianToUint64(powerBytes)
}

// HasVotingPowerTable checks if the voting power table exists at a given height
func (k Keeper) HasVotingPowerTable(ctx context.Context, height uint64) bool {
	store := k.votingPowerBbnBlockHeightStore(ctx, height)
	iter := store.Iterator(nil, nil)
	defer iter.Close()
	return iter.Valid()
}

// GetVotingPowerTable gets the voting power table, i.e., finality provider set at a given height
func (k Keeper) GetVotingPowerTable(ctx context.Context, height uint64) map[string]uint64 {
	store := k.votingPowerBbnBlockHeightStore(ctx, height)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	// if no finality provider at this height, return nil
	if !iter.Valid() {
		return nil
	}

	// get all finality providers at this height
	fpSet := map[string]uint64{}
	for ; iter.Valid(); iter.Next() {
		fpBTCPK, err := bbn.NewBIP340PubKey(iter.Key())
		if err != nil {
			// failing to unmarshal finality provider BTC PK in KVStore is a programming error
			panic(fmt.Errorf("%w: %w", bbn.ErrUnmarshal, err))
		}
		fpSet[fpBTCPK.MarshalHex()] = sdk.BigEndianToUint64(iter.Value())
	}

	return fpSet
}

// GetBTCStakingActivatedHeight returns the height when the BTC staking protocol is activated
// i.e., the first height where a finality provider has voting power
// Before the BTC staking protocol is activated, we don't index or tally any block
func (k Keeper) GetBTCStakingActivatedHeight(ctx context.Context) (uint64, error) {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	votingPowerStore := prefix.NewStore(storeAdapter, types.VotingPowerKey)
	iter := votingPowerStore.Iterator(nil, nil)
	defer iter.Close()
	// if the iterator is valid, then there exists a height that has a finality provider with voting power
	if iter.Valid() {
		return sdk.BigEndianToUint64(iter.Key()), nil
	} else {
		return 0, types.ErrBTCStakingNotActivated
	}
}

func (k Keeper) IsBTCStakingActivated(ctx context.Context) bool {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	votingPowerStore := prefix.NewStore(storeAdapter, types.VotingPowerKey)
	iter := votingPowerStore.Iterator(nil, nil)
	defer iter.Close()
	// if the iterator is valid, then BTC staking is already activated
	return iter.Valid()
}

// votingPowerBbnBlockHeightStore returns the KVStore of the finality providers' voting power
// prefix: (VotingPowerKey || Babylon block height)
// key: Bitcoin secp256k1 PK
// value: voting power quantified in Satoshi
func (k Keeper) votingPowerBbnBlockHeightStore(ctx context.Context, height uint64) prefix.Store {
	votingPowerStore := k.votingPowerStore(ctx)
	return prefix.NewStore(votingPowerStore, sdk.Uint64ToBigEndian(height))
}

// votingPowerStore returns the KVStore of the finality providers' voting power
// prefix: (VotingPowerKey)
// key: Babylon block height || Bitcoin secp256k1 PK
// value: voting power quantified in Satoshi
func (k Keeper) votingPowerStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.VotingPowerKey)
}

func (k Keeper) newVotingPowerStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.VotingPowerAsListKey)
}

func (k Keeper) SetVotingPowerAsList(ctx context.Context, height uint64, activeFPs []*types.ActiveFinalityProvider) {
	store := k.newVotingPowerStore(ctx)
	activeList := types.ActiveFinalityProvidersList{Fps: activeFPs}
	activeListBytes := k.cdc.MustMarshal(&activeList)

	store.Set(sdk.Uint64ToBigEndian(height), activeListBytes)
}

func (k Keeper) GetVotingPowerAsList(ctx context.Context, height uint64) map[string]uint64 {
	store := k.newVotingPowerStore(ctx)
	activeListBytes := store.Get(sdk.Uint64ToBigEndian(height))

	if activeListBytes == nil {
		return nil
	}

	activeList := types.ActiveFinalityProvidersList{}
	k.cdc.MustUnmarshal(activeListBytes, &activeList)

	fpSet := make(map[string]uint64)
	for _, fp := range activeList.Fps {
		fpSet[fp.FpBtcPk.MarshalHex()] = fp.VotingPower
	}

	return fpSet
}

func (k Keeper) GetVotingPowerNew(ctx context.Context, fpBTCPK []byte, height uint64) uint64 {
	store := k.newVotingPowerStore(ctx)
	activeListBytes := store.Get(sdk.Uint64ToBigEndian(height))

	if activeListBytes == nil {
		return 0
	}

	activeList := types.ActiveFinalityProvidersList{}
	k.cdc.MustUnmarshal(activeListBytes, &activeList)

	for _, fp := range activeList.Fps {
		if bytes.Equal(fp.FpBtcPk.MustMarshal(), fpBTCPK) {
			return fp.VotingPower
		}
	}

	return 0
}
