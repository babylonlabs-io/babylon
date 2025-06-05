package types

import (
	"context"

	"cosmossdk.io/x/feegrant"
	sdk "github.com/cosmos/cosmos-sdk/types"

	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
)

type AccountKeeper interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	GetModuleAccount(ctx context.Context, name string) sdk.ModuleAccountI
}

type BankKeeper interface {
	SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromModuleToModule(ctx context.Context, senderModule string, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

type EpochingKeeper interface {
	GetEpoch(ctx context.Context) *epochingtypes.Epoch
}

type FeegrantKeeper interface {
	GrantAllowance(ctx context.Context, granter, grantee sdk.AccAddress, feeAllowance feegrant.FeeAllowanceI) error
	GetAllowance(ctx context.Context, granter, grantee sdk.AccAddress) (feegrant.FeeAllowanceI, error)
	UpdateAllowance(ctx context.Context, granter, grantee sdk.AccAddress, feeAllowance feegrant.FeeAllowanceI) error
}
