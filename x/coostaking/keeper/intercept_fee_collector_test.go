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
)

var (
	fees = sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100)))
)

func FuzzInterceptFeeCollector(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bankK := types.NewMockBankKeeper(ctrl)
		bankK.EXPECT().GetAllBalances(gomock.Any(), appparams.AccFeeCollector).Return(fees).Times(1)

		accK := types.NewMockAccountKeeper(ctrl)
		accK.EXPECT().GetModuleAccount(gomock.Any(), authtypes.FeeCollectorName).Return(appparams.AccFeeCollector).Times(1)

		k, ctx := testkeeper.CoostakingKeeperWithStoreKey(t, nil, bankK, accK)

		// mock (thus ensure) that fees with the exact portion is intercepted
		// NOTE: if the actual fees are different from feesForIncentive the test will fail
		params := k.GetParams(ctx)
		coostakingPortion := ictvtypes.GetCoinsPortion(fees, params.CoostakingPortion)
		bankK.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), gomock.Eq(authtypes.FeeCollectorName), gomock.Eq(types.ModuleName), gomock.Eq(coostakingPortion)).Times(1)

		// handle coins in fee collector
		k.HandleCoinsInFeeCollector(ctx)
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
	accK.EXPECT().GetModuleAccount(gomock.Any(), authtypes.FeeCollectorName).Return(appparams.AccFeeCollector).Times(1)

	keeper, ctx := testkeeper.CoostakingKeeperWithStoreKey(t, nil, bankK, accK)

	// mock (thus ensure) that fees with the exact portion is intercepted
	// NOTE: if the actual fees are different from feesForIncentive the test will fail
	params := keeper.GetParams(ctx)
	feesForBTCStaking := ictvtypes.GetCoinsPortion(smallFee, params.CoostakingPortion)
	bankK.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), gomock.Eq(authtypes.FeeCollectorName), gomock.Eq(types.ModuleName), gomock.Eq(feesForBTCStaking)).Times(1)

	// handle coins in fee collector
	keeper.HandleCoinsInFeeCollector(ctx)
}
