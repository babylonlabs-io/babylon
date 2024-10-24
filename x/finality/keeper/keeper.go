package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/x/finality/types"
)

type (
	Keeper struct {
		cdc          codec.BinaryCodec
		storeService corestoretypes.KVStoreService

		BTCStakingKeeper    types.BTCStakingKeeper
		IncentiveKeeper     types.IncentiveKeeper
		CheckpointingKeeper types.CheckpointingKeeper
		// the address capable of executing a MsgUpdateParams message. Typically, this
		// should be the x/gov module account.
		authority string

		// FinalityProviderSigningTracker key: BIP340PubKey bytes | value: FinalityProviderSigningInfo
		FinalityProviderSigningTracker collections.Map[[]byte, types.FinalityProviderSigningInfo]
		// FinalityProviderMissedBlockBitmap key: BIP340PubKey bytes | value: byte key for a finality provider's missed block bitmap chunk
		FinalityProviderMissedBlockBitmap collections.Map[collections.Pair[[]byte, uint64], []byte]
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestoretypes.KVStoreService,
	btcstakingKeeper types.BTCStakingKeeper,
	incentiveKeeper types.IncentiveKeeper,
	checkpointingKeeper types.CheckpointingKeeper,
	authority string,
) Keeper {
	sb := collections.NewSchemaBuilder(storeService)
	return Keeper{
		cdc:          cdc,
		storeService: storeService,

		BTCStakingKeeper:    btcstakingKeeper,
		IncentiveKeeper:     incentiveKeeper,
		CheckpointingKeeper: checkpointingKeeper,
		authority:           authority,
		FinalityProviderSigningTracker: collections.NewMap(
			sb,
			types.FinalityProviderSigningInfoKeyPrefix,
			"finality_provider_signing_info",
			collections.BytesKey,
			codec.CollValue[types.FinalityProviderSigningInfo](cdc),
		),
		FinalityProviderMissedBlockBitmap: collections.NewMap(
			sb,
			types.FinalityProviderMissedBlockBitmapKeyPrefix,
			"finality_provider_missed_block_bitmap",
			collections.PairKeyCodec(collections.BytesKey, collections.Uint64Key),
			collections.BytesValue,
		),
	}
}

func (k Keeper) BeginBlocker(ctx context.Context) error {
	// update voting power distribution
	k.UpdatePowerDist(ctx)

	return nil
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

func (k Keeper) GetLastFinalizedEpoch(ctx context.Context) uint64 {
	return k.CheckpointingKeeper.GetLastFinalizedEpoch(ctx)
}

func (k Keeper) GetCurrentEpoch(ctx context.Context) uint64 {
	currentEpoch := k.CheckpointingKeeper.GetEpoch(ctx)
	if currentEpoch == nil {
		panic("cannot get the current epoch")
	}

	return currentEpoch.EpochNumber
}

// IsFinalityActive returns true if the finality is activated and ready
// to start handling liveness, tally and index blocks.
func (k Keeper) IsFinalityActive(ctx context.Context) (activated bool) {
	if uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height) < k.GetParams(ctx).FinalityActivationHeight {
		return false
	}

	_, err := k.GetBTCStakingActivatedHeight(ctx)
	return err == nil
}
