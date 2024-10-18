package mint_test

import (
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/testutil/helper"
	"github.com/babylonlabs-io/babylon/x/mint"
	minttypes "github.com/babylonlabs-io/babylon/x/mint/types"
	"github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var oneYear = time.Duration(minttypes.NanosecondsPerYear)

func TestInflationRate(t *testing.T) {
	h := helper.NewHelper(t)
	app := h.App
	ctx := sdk.NewContext(app.CommitMultiStore(), types.Header{}, false, log.NewNopLogger())
	genesisTime := app.MintKeeper.GetGenesisTime(ctx).GenesisTime

	yearOneMinusOneSecond := genesisTime.Add(oneYear).Add(-time.Second)
	yearOne := genesisTime.Add(oneYear)
	yearTwo := genesisTime.Add(2 * oneYear)
	yearFifteen := genesisTime.Add(15 * oneYear)
	yearTwenty := genesisTime.Add(20 * oneYear)

	type testCase struct {
		name string
		ctx  sdk.Context
		want math.LegacyDec
	}

	testCases := []testCase{
		{
			name: "inflation rate is 0.08 for year zero",
			ctx:  ctx.WithBlockHeight(1).WithBlockTime(*genesisTime),
			want: math.LegacyMustNewDecFromStr("0.08"),
		},
		{
			name: "inflation rate is 0.08 for year one minus one second",
			ctx:  ctx.WithBlockTime(yearOneMinusOneSecond),
			want: math.LegacyMustNewDecFromStr("0.08"),
		},
		{
			name: "inflation rate is 0.072 for year one",
			ctx:  ctx.WithBlockTime(yearOne),
			want: math.LegacyMustNewDecFromStr("0.072"),
		},
		{
			name: "inflation rate is 0.0648 for year two",
			ctx:  ctx.WithBlockTime(yearTwo),
			want: math.LegacyMustNewDecFromStr("0.0648"),
		},
		{
			name: "inflation rate is 0.01647129056757192 for year fifteen",
			ctx:  ctx.WithBlockTime(yearFifteen),
			want: math.LegacyMustNewDecFromStr("0.01647129056757192"),
		},
		{
			name: "inflation rate is 0.015 for year twenty",
			ctx:  ctx.WithBlockTime(yearTwenty),
			want: math.LegacyMustNewDecFromStr("0.015"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mint.BeginBlocker(tc.ctx, app.MintKeeper)
			got, err := app.MintKeeper.InflationRate(ctx, &minttypes.QueryInflationRateRequest{})
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got.InflationRate)
		})
	}
}

func TestAnnualProvisions(t *testing.T) {
	t.Run("annual provisions are set after the chain starts", func(t *testing.T) {
		h := helper.NewHelper(t)
		a, ctx := h.App, h.Ctx

		assert.False(t, a.MintKeeper.GetMinter(ctx).AnnualProvisions.IsZero())
	})

	t.Run("annual provisions are not updated more than once per year", func(t *testing.T) {
		h := helper.NewHelper(t)
		a, ctx := h.App, h.Ctx

		genesisTime := a.MintKeeper.GetGenesisTime(ctx).GenesisTime
		yearOneMinusOneSecond := genesisTime.Add(oneYear).Add(-time.Second)

		initialSupply, err := a.StakingKeeper.StakingTokenSupply(ctx)
		require.NoError(t, err)
		stakingBondDenom, err := a.StakingKeeper.BondDenom(ctx)
		require.NoError(t, err)
		require.Equal(t, a.MintKeeper.GetMinter(ctx).BondDenom, stakingBondDenom)

		blockInterval := time.Second * 15

		want := minttypes.InitialInflationRateAsDec().MulInt(initialSupply)

		type testCase struct {
			height int64
			time   time.Time
		}
		testCases := []testCase{
			{1, genesisTime.Add(blockInterval)},
			{2, genesisTime.Add(blockInterval * 2)},
			{3, yearOneMinusOneSecond},
			// testing annual provisions for years after year zero depends on the
			// total supply which increased due to inflation in year zero.
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("block height %v", tc.height), func(t *testing.T) {
				ctx = ctx.WithBlockHeight(tc.height).WithBlockTime(tc.time)
				mint.BeginBlocker(ctx, a.MintKeeper)
				assert.True(t, a.MintKeeper.GetMinter(ctx).AnnualProvisions.Equal(want))
			})
		}

		t.Run("one year later", func(t *testing.T) {
			yearOne := genesisTime.Add(oneYear)
			ctx = ctx.WithBlockHeight(5).WithBlockTime(yearOne)
			mint.BeginBlocker(ctx, a.MintKeeper)
			assert.False(t, a.MintKeeper.GetMinter(ctx).AnnualProvisions.Equal(want))
		})
	})
}
