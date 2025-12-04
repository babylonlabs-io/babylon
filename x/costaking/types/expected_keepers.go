package types

import (
	context "context"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type IncentiveKeeper interface {
	AccumulateRewardGaugeForCostaker(ctx context.Context, addr sdk.AccAddress, reward sdk.Coins)
	IterateBTCDelegationSatsUpdated(ctx context.Context, fp sdk.AccAddress, it func(del sdk.AccAddress, activeSats sdkmath.Int) error) error
}

type AccountKeeper interface {
	GetModuleAccount(ctx context.Context, name string) sdk.ModuleAccountI
	GetModuleAddress(moduleName string) sdk.AccAddress
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
	GetValidatorDelegations(ctx context.Context, valAddr sdk.ValAddress) ([]stakingtypes.Delegation, error)
	IterateLastValidatorPowers(ctx context.Context, handler func(operator sdk.ValAddress, power int64) bool) error
	ValidatorByConsAddr(context.Context, sdk.ConsAddress) (stakingtypes.ValidatorI, error)
	GetValidator(ctx context.Context, addr sdk.ValAddress) (validator stakingtypes.Validator, err error)
}

type FinalityKeeper interface {
	GetVotingPowerDistCache(ctx context.Context, height uint64) *ftypes.VotingPowerDistCache
}

type BtcStkKeeper interface {
	HandleFPBTCDelegations(ctx context.Context, fpBTCPK *bbn.BIP340PubKey, handler func(*btcstktypes.BTCDelegation) error) error
	GetParamsByVersion(ctx context.Context, v uint32) *btcstktypes.Params
	BtcTip(ctx context.Context) uint32
}
