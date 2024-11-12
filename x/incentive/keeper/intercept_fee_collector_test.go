package keeper_test

import (
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testkeeper "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
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
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock bank keeper
		bankKeeper := types.NewMockBankKeeper(ctrl)
		bankKeeper.EXPECT().GetAllBalances(gomock.Any(), feeCollectorAcc.GetAddress()).Return(fees).Times(1)

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
		feesForBTCStaking := types.GetCoinsPortion(fees, params.BTCStakingPortion())
		bankKeeper.EXPECT().SendCoinsFromModuleToModule(gomock.Any(), gomock.Eq(authtypes.FeeCollectorName), gomock.Eq(types.ModuleName), gomock.Eq(feesForBTCStaking)).Times(1)

		// handle coins in fee collector
		keeper.HandleCoinsInFeeCollector(ctx)

		// assert correctness of BTC staking gauge at height
		btcStakingFee := types.GetCoinsPortion(fees, params.BTCStakingPortion())
		btcStakingGauge := keeper.GetBTCStakingGauge(ctx, height)
		require.NotNil(t, btcStakingGauge)
		require.Equal(t, btcStakingFee, btcStakingGauge.Coins)
	})
}
