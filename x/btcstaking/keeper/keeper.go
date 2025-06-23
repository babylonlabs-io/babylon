package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	corestoretypes "cosmossdk.io/core/store"

	"cosmossdk.io/log"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

type (
	Keeper struct {
		cdc                     codec.BinaryCodec
		storeService            corestoretypes.KVStoreService
		btcStakingModuleAddress string

		btclcKeeper types.BTCLightClientKeeper
		btccKeeper  types.BtcCheckpointKeeper
		BscKeeper   types.BTCStkConsumerKeeper
		iKeeper     types.IncentiveKeeper

		Schema                       collections.Schema
		AllowedStakingTxHashesKeySet collections.KeySet[[]byte]
		// LargestBtcReorg defines the BTC block height difference between
		// the btc tip height and the rollback block height
		LargestBtcReorg collections.Item[types.LargestBtcReOrg]

		btcNet *chaincfg.Params
		// the address capable of executing a MsgUpdateParams or
		// MsgResumeFinalityProposal message. Typically, this
		// should be the x/gov module account.
		authority string
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestoretypes.KVStoreService,

	btclcKeeper types.BTCLightClientKeeper,
	btccKeeper types.BtcCheckpointKeeper,
	bscKeeper types.BTCStkConsumerKeeper,
	iKeeper types.IncentiveKeeper,

	btcNet *chaincfg.Params,
	btcStakingModuleAddress string,
	authority string,
) Keeper {
	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		cdc:                     cdc,
		storeService:            storeService,
		btcStakingModuleAddress: btcStakingModuleAddress,
		btclcKeeper:             btclcKeeper,
		btccKeeper:              btccKeeper,
		BscKeeper:               bscKeeper,
		iKeeper:                 iKeeper,

		AllowedStakingTxHashesKeySet: collections.NewKeySet(
			sb,
			types.AllowedStakingTxHashesKey,
			"allowed_staking_tx_hashes_key_set",
			collections.BytesKey,
		),
		LargestBtcReorg: collections.NewItem[types.LargestBtcReOrg](
			sb,
			types.LargestBtcReorgInBlocks,
			"largest_btc_reorg",
			codec.CollValue[types.LargestBtcReOrg](cdc),
		),
		btcNet:    btcNet,
		authority: authority,
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// BeginBlocker is invoked upon `BeginBlock` of the system. The function
// iterates over all BTC delegations under non-slashed finality providers
// to 1) record the voting power table for the current height, and 2) record
// the voting power distribution cache used for computing voting power table
// and distributing rewards once the block is finalised by finality providers.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	// index BTC height at the current height
	k.IndexBTCHeight(ctx)

	return nil
}

func (k Keeper) BtccKeeper() types.BtcCheckpointKeeper {
	if k.btccKeeper == nil {
		panic("BtcCheckpointKeeper is not set")
	}
	return k.btccKeeper
}
