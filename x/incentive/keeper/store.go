package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) GetWithdrawAddr(ctx context.Context, addr sdk.AccAddress) (sdk.AccAddress, error) {
	store := k.storeService.OpenKVStore(ctx)
	b, err := store.Get(types.GetWithdrawAddrKey(addr))
	if b == nil {
		return addr, err
	}

	return b, nil
}

func (k Keeper) SetWithdrawAddr(ctx context.Context, addr, withdrawAddr sdk.AccAddress) error {
	store := k.storeService.OpenKVStore(ctx)

	return store.Set(types.GetWithdrawAddrKey(addr), withdrawAddr.Bytes())
}
