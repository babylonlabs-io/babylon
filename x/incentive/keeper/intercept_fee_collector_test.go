package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	feeCollectorAcc = authtypes.NewEmptyModuleAccount(authtypes.FeeCollectorName)
	fees            = sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100)))
)

func FuzzInterceptFeeCollector(f *testing.F) {
	noFeesSeed := int64(99999999)
	f.Add(noFeesSeed) // special case with no fees

	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		bankKeeper := types.NewMockBankKeeper(ctrl)
		accountKeeper := types.NewMockAccountKeeper(ctrl)
		epochingKeeper := types.NewMockEpochingKeeper(ctrl)

		accountKeeper.EXPECT().GetModuleAccount(gomock.Any(), authtypes.FeeCollectorName).Return(feeCollectorAcc).Times(1)

		keeper, ctx := testkeeper.IncentiveKeeper(t, bankKeeper, accountKeeper, epochingKeeper)

		params := keeper.GetParams(ctx)

		var height uint64
		if seed == noFeesSeed {
			height = 100_000 // unique height for empty-fees test
			ctx = datagen.WithCtxHeight(ctx, height)

			bankKeeper.EXPECT().GetAllBalances(gomock.Any(), feeCollectorAcc.GetAddress()).Return(sdk.NewCoins()).Times(1)

			// call the function
			keeper.HandleCoinsInFeeCollector(ctx)

			// assert empty gauge is stored at height
			gauge := keeper.GetBTCStakingGauge(ctx, uint64(height))
			require.NotNil(t, gauge)
			require.True(t, gauge.Coins.IsZero())
		} else {
			height = datagen.RandomIntOtherThan(r, 100_000, 1000) // avoid 100_000
			ctx = datagen.WithCtxHeight(ctx, height)

			bankKeeper.EXPECT().GetAllBalances(gomock.Any(), feeCollectorAcc.GetAddress()).Return(fees).Times(1)

			feesForBTCStaking := types.GetCoinsPortion(fees, params.BTCStakingPortion())
			bankKeeper.EXPECT().
				SendCoinsFromModuleToModule(gomock.Any(), authtypes.FeeCollectorName, types.ModuleName, feesForBTCStaking).
				Times(1)

			keeper.HandleCoinsInFeeCollector(ctx)

			gauge := keeper.GetBTCStakingGauge(ctx, uint64(height))
			require.NotNil(t, gauge)
			require.Equal(t, feesForBTCStaking, gauge.Coins)
		}
	})
}

func TestInterceptFeeCollectorWithSmallAmount(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	smallFee := sdk.NewCoins(
		sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(1)),
		sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(10000)),
	)

	// mock bank keeper
	bankKeeper := types.NewMockBankKeeper(ctrl)
	bankKeeper.EXPECT().GetAllBalances(gomock.Any(), feeCollectorAcc.GetAddress()).Return(smallFee).Times(1)

	// mock account keeper
	accountKeeper := types.NewMockAccountKeeper(ctrl)
	accountKeeper.EXPECT().GetModuleAccount(gomock.Any(), authtypes.FeeCollectorName).Return(feeCollectorAcc).Times(1)

	// mock epoching keeper
	epochingKeeper := types.NewMockEpochingKeeper(ctrl)

	keeper, ctx := testkeeper.IncentiveKeeper(t, bankKeeper, accountKeeper, epochingKeeper)
	height := datagen.RandomInt(r, 1000)
	ctx = datagen.WithCtxHeight(ctx, height)

	// mock (thus ensure) that fees with the exact portion is intercepted
	// NOTE: if the actual fees are different from feesForIncentive the test will fail
	params := keeper.GetParams(ctx)
	feesForBTCStaking := types.GetCoinsPortion(smallFee, params.BTCStakingPortion())
	bankKeeper.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), gomock.Eq(authtypes.FeeCollectorName), gomock.Eq(types.ModuleName), gomock.Eq(feesForBTCStaking)).Times(1)

	// handle coins in fee collector
	keeper.HandleCoinsInFeeCollector(ctx)

	// assert correctness of BTC staking gauge at height
	btcStakingFee := types.GetCoinsPortion(smallFee, params.BTCStakingPortion())
	btcStakingGauge := keeper.GetBTCStakingGauge(ctx, height)
	require.NotNil(t, btcStakingGauge)
	require.Equal(t, btcStakingFee, btcStakingGauge.Coins)
}
