package keeper

import (
	"context"
	"fmt"

	corestoretypes "cosmossdk.io/core/store"

	"cosmossdk.io/log"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

type (
	Keeper struct {
		cdc          codec.BinaryCodec
		storeService corestoretypes.KVStoreService

		btclcKeeper    types.BTCLightClientKeeper
		btccKeeper     types.BtcCheckpointKeeper
		FinalityKeeper types.FinalityKeeper
		iKeeper        types.IncentiveKeeper

		hooks types.BtcStakingHooks

		btcNet *chaincfg.Params
		// the address capable of executing a MsgUpdateParams message. Typically, this
		// should be the x/gov module account.
		authority string
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestoretypes.KVStoreService,

	btclcKeeper types.BTCLightClientKeeper,
	btccKeeper types.BtcCheckpointKeeper,
	finalityKeeper types.FinalityKeeper,
	iKeeper types.IncentiveKeeper,

	btcNet *chaincfg.Params,
	authority string,
) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,

		btclcKeeper:    btclcKeeper,
		btccKeeper:     btccKeeper,
		FinalityKeeper: finalityKeeper,
		iKeeper:        iKeeper,

		hooks: nil,

		btcNet:    btcNet,
		authority: authority,
	}
}

// SetHooks sets the BTC staking hooks
func (k *Keeper) SetHooks(sh types.BtcStakingHooks) *Keeper {
	if k.hooks != nil {
		panic("cannot set BTC staking hooks twice")
	}

	k.hooks = sh

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
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	activationHeight := k.GetActivationHeight()
	currHeight := sdkCtx.HeaderInfo().Height
	if currHeight < activationHeight {
		// TODO: remove it after Phase-2 launch in a future
		// coordinated upgrade
		k.Logger(sdkCtx).With(
			"currHeight", currHeight,
			"activationHeight", activationHeight,
		).Info("BTC staking is not activated yet")
		return nil
	}

	// index BTC height at the current height
	k.IndexBTCHeight(ctx)
	// update voting power distribution
	k.UpdatePowerDist(ctx)

	return nil
}

// GetActivationHeight returns the minimum block height from which the BTC
// staking protocol starts updating the voting power table.
func (k Keeper) GetActivationHeight() int64 {
	switch k.btcNet.Name {
	// The activation height might differ accordingly
	// with the test that will be done and the block time
	// that the validator sets in config.
	case chaincfg.MainNetParams.Name:
		// For mainnet considering we want 48 hours
		// at a block time of 10s that would be 17280 blocks
		// considering the upgrade for Phase-2 will happen at block
		// 220, the mainnet activation height for btcstaking should
		// be 17280 + 220.
		return 17500
	case chaincfg.SigNetParams.Name:
		// Signet is used by the devnet testing and 50 blocks
		// is enough to execute proper checks and verify the voting
		// power table is not activated before this.
		return 50
	case chaincfg.RegressionNetParams.Name:
		// regtest is only used for internal deployment testing
		// and 40 blocks is enough to do the proper checks before
		// the height is reached.
		return 40
	default:
		// Overall btc network configs do not need to wait for activation
		// as unit tests should not be affected by this.
		return 0
	}
}
