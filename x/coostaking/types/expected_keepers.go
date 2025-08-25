package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type AccountKeeper interface {
	GetModuleAccount(ctx context.Context, name string) sdk.ModuleAccountI
}

type BankKeeper interface {
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromModuleToModule(ctx context.Context, senderModule string, recipientModule string, amt sdk.Coins) error
}
