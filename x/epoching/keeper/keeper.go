package keeper

import (
	"fmt"

	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	"github.com/cosmos/cosmos-sdk/codec"
)

type (
	Keeper struct {
		cdc          codec.BinaryCodec
		storeService corestoretypes.KVStoreService
		hooks        types.EpochingHooks
		bk           types.BankKeeper
		stk          types.StakingKeeper
		stkMsgServer stktypes.MsgServer
		// the address capable of executing a MsgUpdateParams message. Typically, this
		// should be the x/gov module account.
		authority string
	}
)

// ValidateDelegatePoolAccount validates that the delegation pool module account exists
// This should be called before NewKeeper to ensure proper setup
func ValidateDelegatePoolAccount(ak types.AccountKeeper) {
	if addr := ak.GetModuleAddress(types.DelegatePoolModuleName); addr == nil {
		panic(fmt.Sprintf("the %s module account has not been set", types.DelegatePoolModuleName))
	}
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestoretypes.KVStoreService,
	bk types.BankKeeper,
	stk types.StakingKeeper,
	stkMsgServer stktypes.MsgServer,
	authority string,
) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		hooks:        nil,
		bk:           bk,
		stk:          stk,
		stkMsgServer: stkMsgServer,
		authority:    authority,
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// SetHooks sets the validator hooks
func (k *Keeper) SetHooks(eh types.EpochingHooks) *Keeper {
	if k.hooks != nil {
		panic("cannot set validator hooks twice")
	}

	k.hooks = eh

	return k
}
