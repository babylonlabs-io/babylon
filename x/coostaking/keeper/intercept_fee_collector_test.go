package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/coostaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
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

		fees := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100)))
		bankK := types.NewMockBankKeeper(ctrl)
		bankK.EXPECT().GetAllBalances(gomock.Any(), appparams.AccFeeCollector).Return(fees).Times(1)

		accK := types.NewMockAccountKeeper(ctrl)
		accK.EXPECT().GetModuleAccount(gomock.Any(), authtypes.FeeCollectorName).Return(feeCollectorAcc).Times(1)

		k, ctx := testkeeper.CoostakingKeeperWithStoreKey(t, nil, bankK, accK)

		// mock (thus ensure) that fees with the exact portion is intercepted
		// NOTE: if the actual fees are different from feesForIncentive the test will fail
		params := k.GetParams(ctx)
		coostakingPortion := ictvtypes.GetCoinsPortion(fees, params.CoostakingPortion)
		bankK.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), gomock.Eq(authtypes.FeeCollectorName), gomock.Eq(types.ModuleName), gomock.Eq(coostakingPortion)).Times(1)

		// handle coins in fee collector
		err := k.HandleCoinsInFeeCollector(ctx)
		require.NoError(t, err)

		rwd, err := k.GetCurrentRewards(ctx)
		require.NoError(t, err)
		require.Equal(t, coostakingPortion.MulInt(ictvtypes.DecimalRewards).String(), rwd.Rewards.String())
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

	k, ctx := testkeeper.CoostakingKeeperWithStoreKey(t, nil, bankK, accK)

	// mock (thus ensure) that fees with the exact portion is intercepted
	// NOTE: if the actual fees are different the test will fail
	params := k.GetParams(ctx)
	coostakingPortion := ictvtypes.GetCoinsPortion(smallFee, params.CoostakingPortion)
	bankK.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), gomock.Eq(authtypes.FeeCollectorName), gomock.Eq(types.ModuleName), gomock.Eq(coostakingPortion)).Times(1)

	// handle coins in fee collector
	err := k.HandleCoinsInFeeCollector(ctx)
	require.NoError(t, err)

	rwd, err := k.GetCurrentRewards(ctx)
	require.NoError(t, err)
	require.Equal(t, coostakingPortion.MulInt(ictvtypes.DecimalRewards).String(), rwd.Rewards.String())
	require.Equal(t, rwd.Period, uint64(1))
	require.Equal(t, rwd.TotalScore.String(), sdkmath.ZeroInt().String())
}
