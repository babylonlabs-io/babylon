package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/babylonlabs-io/babylon/v4/x/mint/types"
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

	Schema           collections.Schema
	MinterStore      collections.Item[types.Minter]
	GenesisTimeStore collections.Item[types.GenesisTime]
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

	sb := collections.NewSchemaBuilder(storeService)
	k := Keeper{
		cdc:              cdc,
		storeService:     storeService,
		stakingKeeper:    stakingKeeper,
		bankKeeper:       bankKeeper,
		feeCollectorName: feeCollectorName,
		authority:        authority,
		MinterStore:      collections.NewItem(sb, types.MinterKey, "minter", codec.CollValue[types.Minter](cdc)),
		GenesisTimeStore: collections.NewItem(sb, types.GenesisTimeKey, "genesis_time", codec.CollValue[types.GenesisTime](cdc)),
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

// GetMinter returns the minter.
func (k Keeper) GetMinter(ctx context.Context) types.Minter {
	minter, err := k.MinterStore.Get(ctx)
	if err != nil {
		panic(err)
	}
	return minter
}

// SetMinter sets the minter.
func (k Keeper) SetMinter(ctx context.Context, minter types.Minter) error {
	return k.MinterStore.Set(ctx, minter)
}

// GetGenesisTime returns the genesis time.
func (k Keeper) GetGenesisTime(ctx context.Context) (gt types.GenesisTime) {
	genesisTime, err := k.GenesisTimeStore.Get(ctx)
	if err != nil {
		panic(err)
	}
	return genesisTime
}

// SetGenesisTime sets the genesis time.
func (k Keeper) SetGenesisTime(ctx context.Context, gt types.GenesisTime) error {
	return k.GenesisTimeStore.Set(ctx, gt)
}

// StakingTokenSupply implements an alias call to the underlying staking keeper's
// StakingTokenSupply.
func (k Keeper) StakingTokenSupply(ctx context.Context) (math.Int, error) {
	return k.stakingKeeper.StakingTokenSupply(ctx)
}

// MintCoins implements an alias call to the underlying bank keeper's
// MintCoins.
func (k Keeper) MintCoins(ctx context.Context, newCoins sdk.Coins) error {
	if newCoins.Empty() {
		return nil
	}

	return k.bankKeeper.MintCoins(ctx, types.ModuleName, newCoins)
}

// SendCoinsToFeeCollector sends newly minted coins from the x/mint module to
// the x/auth fee collector module account.
func (k Keeper) SendCoinsToFeeCollector(ctx context.Context, coins sdk.Coins) error {
	return k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, k.feeCollectorName, coins)
}
