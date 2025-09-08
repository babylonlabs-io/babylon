package types

import (
	context "context"

	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type IncentiveKeeper interface {
	AccumulateRewardGaugeForCostaker(ctx context.Context, addr sdk.AccAddress, reward sdk.Coins)
	IterateBTCDelegationRewardsTracker(ctx context.Context, fp sdk.AccAddress, it func(fp, del sdk.AccAddress, val ictvtypes.BTCDelegationRewardsTracker) error) error
}

type AccountKeeper interface {
	GetModuleAccount(ctx context.Context, name string) sdk.ModuleAccountI
}

type BankKeeper interface {
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromModuleToModule(ctx context.Context, senderModule string, recipientModule string, amt sdk.Coins) error
}

type DistributionKeeper interface {
	AllocateTokensToValidator(ctx context.Context, val stakingtypes.ValidatorI, tokens sdk.DecCoins) error
}

// StakingKeeper expected staking keeper (noalias)
type StakingKeeper interface {
	GetDelegation(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (stakingtypes.Delegation, error)
	ValidatorByConsAddr(context.Context, sdk.ConsAddress) (stakingtypes.ValidatorI, error)
	Validator(context.Context, sdk.ValAddress) (stakingtypes.ValidatorI, error)
}
