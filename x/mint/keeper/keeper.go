package keeper

import (
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/babylonlabs-io/babylon/x/mint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Keeper of the mint store
type Keeper struct {
	cdc              codec.BinaryCodec
	storeService     store.KVStoreService
	stakingKeeper    types.StakingKeeper
	bankKeeper       types.BankKeeper
	feeCollectorName string
	// the address capable of executing a MsgUpdateParams message. Typically, this
	// should be the x/gov module account.
	authority string
}

// NewKeeper creates a new mint Keeper instance.
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	stakingKeeper types.StakingKeeper,
	ak types.AccountKeeper,
	bankKeeper types.BankKeeper,
	feeCollectorName string,
	authority string,
) Keeper {
	// Ensure the mint module account has been set
	if addr := ak.GetModuleAddress(types.ModuleName); addr == nil {
		panic("the mint module account has not been set")
	}

	return Keeper{
		cdc:              cdc,
		storeService:     storeService,
		stakingKeeper:    stakingKeeper,
		bankKeeper:       bankKeeper,
		feeCollectorName: feeCollectorName,
		authority:        authority,
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// GetMinter returns the minter.
func (k Keeper) GetMinter(ctx sdk.Context) (minter types.Minter) {
	store := k.storeService.OpenKVStore(ctx)
	b, err := store.Get(types.KeyMinter)
	if err != nil {
		panic(err)
	}

	k.cdc.MustUnmarshal(b, &minter)
	return minter
}

// SetMinter sets the minter.
func (k Keeper) SetMinter(ctx sdk.Context, minter types.Minter) error {
	store := k.storeService.OpenKVStore(ctx)
	b := k.cdc.MustMarshal(&minter)
	return store.Set(types.KeyMinter, b)
}

// GetGenesisTime returns the genesis time.
func (k Keeper) GetGenesisTime(ctx sdk.Context) (gt types.GenesisTime) {
	store := k.storeService.OpenKVStore(ctx)
	b, err := store.Get(types.KeyGenesisTime)
	if err != nil {
		panic(err)
	}
	if b == nil {
		panic("stored genesis time should not have been nil")
	}

	k.cdc.MustUnmarshal(b, &gt)
	return gt
}

// SetGenesisTime sets the genesis time.
func (k Keeper) SetGenesisTime(ctx sdk.Context, gt types.GenesisTime) error {
	store := k.storeService.OpenKVStore(ctx)
	b := k.cdc.MustMarshal(&gt)
	return store.Set(types.KeyGenesisTime, b)
}

// StakingTokenSupply implements an alias call to the underlying staking keeper's
// StakingTokenSupply.
func (k Keeper) StakingTokenSupply(ctx sdk.Context) (math.Int, error) {
	return k.stakingKeeper.StakingTokenSupply(ctx)
}

// MintCoins implements an alias call to the underlying bank keeper's
// MintCoins.
func (k Keeper) MintCoins(ctx sdk.Context, newCoins sdk.Coins) error {
	if newCoins.Empty() {
		return nil
	}

	return k.bankKeeper.MintCoins(ctx, types.ModuleName, newCoins)
}

// SendCoinsToFeeCollector sends newly minted coins from the x/mint module to
// the x/auth fee collector module account.
func (k Keeper) SendCoinsToFeeCollector(ctx sdk.Context, coins sdk.Coins) error {
	return k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, k.feeCollectorName, coins)
}
