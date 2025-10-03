package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	cmttypes "github.com/cometbft/cometbft/abci/types"
	tmtypes "github.com/cometbft/cometbft/proto/tendermint/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	feeCollectorAcc = authtypes.NewEmptyModuleAccount(authtypes.FeeCollectorName)
)

func FuzzInterceptFeeCollector(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		fees := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100000000000)))
		bankK := types.NewMockBankKeeper(ctrl)
		bankK.EXPECT().GetAllBalances(gomock.Any(), appparams.AccFeeCollector).Return(fees).Times(1)

		accK := types.NewMockAccountKeeper(ctrl)
		accK.EXPECT().GetModuleAccount(gomock.Any(), authtypes.FeeCollectorName).Return(feeCollectorAcc).Times(1)
		accK.EXPECT().GetModuleAddress(gomock.Any()).Return(authtypes.NewModuleAddress(types.ModuleName)).AnyTimes()

		stkK := types.NewMockStakingKeeper(ctrl)
		distK := types.NewMockDistributionKeeper(ctrl)
		epochK := types.NewMockEpochingKeeper(ctrl)

		k, ctx := testkeeper.CostakingKeeperWithStoreKey(t, nil, bankK, accK, nil, stkK, distK, epochK)

		// Create a mock validator
		consAddr := sdk.ConsAddress([]byte("validator1"))
		validator := stakingtypes.Validator{
			OperatorAddress: "validator1",
			ConsensusPubkey: nil,
		}

		// Add vote info to context
		voteInfos := []cmttypes.VoteInfo{
			{
				Validator: cmttypes.Validator{
					Address: consAddr,
					Power:   100,
				},
				BlockIdFlag: tmtypes.BlockIDFlagCommit,
			},
		}
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		ctxWithVotes := sdkCtx.WithVoteInfos(voteInfos)

		// mock (thus ensure) that fees with the exact portion is intercepted
		// NOTE: if the actual fees are different from feesForIncentive the test will fail
		params := k.GetParams(ctx)
		validatorsPortion := ictvtypes.GetCoinsPortion(fees, params.ValidatorsPortion)
		require.True(t, validatorsPortion.IsAllPositive())
		stkK.EXPECT().ValidatorByConsAddr(gomock.Any(), consAddr).Return(validator, nil).Times(1)
		bankK.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), gomock.Eq(authtypes.FeeCollectorName), gomock.Eq(disttypes.ModuleName), gomock.Eq(validatorsPortion)).Times(1)
		validator.Commission.Rate = sdkmath.LegacyOneDec()
		distK.EXPECT().AllocateTokensToValidator(gomock.Any(), validator, sdk.NewDecCoinsFromCoins(validatorsPortion...)).Return(nil).Times(1)

		costakingPortion := ictvtypes.GetCoinsPortion(fees, params.CostakingPortion)
		bankK.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), gomock.Eq(authtypes.FeeCollectorName), gomock.Eq(types.ModuleName), gomock.Eq(costakingPortion)).Times(1)

		// handle coins in fee collector
		err := k.HandleCoinsInFeeCollector(ctxWithVotes)
		require.NoError(t, err)

		rwd, err := k.GetCurrentRewards(ctxWithVotes)
		require.NoError(t, err)
		require.Equal(t, costakingPortion.MulInt(ictvtypes.DecimalRewards).String(), rwd.Rewards.String())
		require.Equal(t, rwd.Period, uint64(1))
		require.Equal(t, rwd.TotalScore.String(), sdkmath.ZeroInt().String())
	})
}

func TestInterceptFeeCollectorWithSmallAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	smallFee := sdk.NewCoins(
		sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1)),
		sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(10000)),
	)

	bankK := types.NewMockBankKeeper(ctrl)
	bankK.EXPECT().GetAllBalances(gomock.Any(), appparams.AccFeeCollector).Return(smallFee).Times(1)

	accK := types.NewMockAccountKeeper(ctrl)
	accK.EXPECT().GetModuleAccount(gomock.Any(), authtypes.FeeCollectorName).Return(feeCollectorAcc).Times(1)
	accK.EXPECT().GetModuleAddress(gomock.Any()).Return(authtypes.NewModuleAddress(types.ModuleName)).AnyTimes()

	stkK := types.NewMockStakingKeeper(ctrl)
	distK := types.NewMockDistributionKeeper(ctrl)
	epochK := types.NewMockEpochingKeeper(ctrl)

	k, ctx := testkeeper.CostakingKeeperWithStoreKey(t, nil, bankK, accK, nil, stkK, distK, epochK)

	// Create a mock validator
	consAddr := sdk.ConsAddress([]byte("validator1"))
	validator := stakingtypes.Validator{
		OperatorAddress: "validator1",
		ConsensusPubkey: nil,
	}

	// Add vote info to context
	voteInfos := []cmttypes.VoteInfo{
		{
			Validator: cmttypes.Validator{
				Address: consAddr,
				Power:   100,
			},
			BlockIdFlag: tmtypes.BlockIDFlagCommit,
		},
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctxWithVotes := sdkCtx.WithVoteInfos(voteInfos)

	// mock (thus ensure) that fees with the exact portion is intercepted
	// NOTE: if the actual fees are different the test will fail
	params := k.GetParams(ctx)

	validatorsPortion := ictvtypes.GetCoinsPortion(smallFee, params.ValidatorsPortion)
	require.True(t, validatorsPortion.IsAllPositive())
	stkK.EXPECT().ValidatorByConsAddr(gomock.Any(), consAddr).Return(validator, nil).Times(1)
	bankK.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), gomock.Eq(authtypes.FeeCollectorName), gomock.Eq(disttypes.ModuleName), gomock.Eq(validatorsPortion)).Times(1)
	validator.Commission.Rate = sdkmath.LegacyOneDec()
	distK.EXPECT().AllocateTokensToValidator(gomock.Any(), validator, sdk.NewDecCoinsFromCoins(validatorsPortion...)).Return(nil).Times(1)

	costakingPortion := ictvtypes.GetCoinsPortion(smallFee, params.CostakingPortion)
	bankK.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), gomock.Eq(authtypes.FeeCollectorName), gomock.Eq(types.ModuleName), gomock.Eq(costakingPortion)).Times(1)

	// handle coins in fee collector
	err := k.HandleCoinsInFeeCollector(ctxWithVotes)
	require.NoError(t, err)

	rwd, err := k.GetCurrentRewards(ctxWithVotes)
	require.NoError(t, err)
	require.Equal(t, costakingPortion.MulInt(ictvtypes.DecimalRewards).String(), rwd.Rewards.String())
	require.Equal(t, rwd.Period, uint64(1))
	require.Equal(t, rwd.TotalScore.String(), sdkmath.ZeroInt().String())
}
