package types_test

import (
	"math/rand"
	"testing"
	time "time"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/mint/types"
)

func TestCalculateInflationRate(t *testing.T) {
	minter := types.DefaultMinter()
	genesisTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		year int64
		want float64
	}

	testCases := []testCase{
		{0, 0.055},
		{1, 0.055},
		{2, 0.055},
		{3, 0.055},
		{4, 0.055},
		{5, 0.055},
		{6, 0.055},
		{7, 0.055},
		{8, 0.055},
		{9, 0.055},
		{10, 0.055},
		{11, 0.055},
		{12, 0.055},
		{13, 0.055},
		{14, 0.055},
		{15, 0.055},
		{16, 0.055},
		{17, 0.055},
		{18, 0.055},
		{19, 0.055},
		{20, 0.055},
		{21, 0.055},
		{22, 0.055},
		{23, 0.055},
		{24, 0.055},
		{25, 0.055},
		{26, 0.055},
		{27, 0.055},
		{28, 0.055},
		{29, 0.055},
		{30, 0.055},
		{31, 0.055},
		{32, 0.055},
		{33, 0.055},
		{34, 0.055},
		{35, 0.055},
		{36, 0.055},
		{37, 0.055},
		{38, 0.055},
		{39, 0.055},
		{40, 0.055},
	}

	for _, tc := range testCases {
		years := time.Duration(tc.year * types.NanosecondsPerYear * int64(time.Nanosecond))
		blockTime := genesisTime.Add(years)
		ctx := sdk.NewContext(nil, tmproto.Header{}, false, nil).WithBlockTime(blockTime)
		inflationRate := minter.CalculateInflationRate(ctx, genesisTime)
		got, err := inflationRate.Float64()
		assert.NoError(t, err)
		assert.Equal(t, tc.want, got, "want %v got %v year %v blockTime %v", tc.want, got, tc.year, blockTime)
	}
}

func TestCalculateBlockProvision(t *testing.T) {
	minter := types.DefaultMinter()
	current := time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)
	blockInterval := 15 * time.Second
	totalSupply := math.LegacyNewDec(1_000_000_000_000)                    // 1 trillion ubbn
	annualProvisions := totalSupply.Mul(types.InitialInflationRateAsDec()) // 80 billion ubbn

	type testCase struct {
		name             string
		annualProvisions math.LegacyDec
		current          time.Time
		previous         time.Time
		want             sdk.Coin
		wantErr          bool
	}

	testCases := []testCase{
		{
			name:             "one 15 second block during the first year",
			annualProvisions: annualProvisions,
			current:          current,
			previous:         current.Add(-blockInterval),
			// 55 billion ubbn (annual provisions) * 15 (seconds) / 31,556,952 (seconds per year) = 26143.209268119430545763735356950823387505865585498 which truncates to 26143 ubbn
			want: sdk.NewCoin(types.DefaultBondDenom, math.NewInt(26143)),
		},
		{
			name:             "one 30 second block during the first year",
			annualProvisions: annualProvisions,
			current:          current,
			previous:         current.Add(-2 * blockInterval),
			// 55 billion ubbn (annual provisions) * 30 (seconds) / 31,556,952 (seconds per year) = 52286.418536238861091527470713901646775011731170995  which truncates to 52.286 ubbn
			want: sdk.NewCoin(types.DefaultBondDenom, math.NewInt(52286)),
		},
		{
			name:             "want error when current time is before previous time",
			annualProvisions: annualProvisions,
			current:          current,
			previous:         current.Add(blockInterval),
			wantErr:          true,
		},
	}
	for _, tc := range testCases {
		minter.AnnualProvisions = tc.annualProvisions
		got, err := minter.CalculateBlockProvision(tc.current, tc.previous)
		if tc.wantErr {
			assert.Error(t, err)
			return
		}
		assert.NoError(t, err)
		require.True(t, tc.want.IsEqual(got), "want %v got %v", tc.want, got)
	}
}

// TestCalculateBlockProvisionError verifies that the error for total block
// provisions in a year is less than .01
func TestCalculateBlockProvisionError(t *testing.T) {
	minter := types.DefaultMinter()
	current := time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)
	oneYear := time.Duration(types.NanosecondsPerYear)
	end := current.Add(oneYear)

	totalSupply := math.LegacyNewDec(1_000_000_000_000)                    // 1 trillion ubbn
	annualProvisions := totalSupply.Mul(types.InitialInflationRateAsDec()) // 80 billion ubbn
	minter.AnnualProvisions = annualProvisions
	totalBlockProvisions := math.LegacyNewDec(0)
	for current.Before(end) {
		blockInterval := randomBlockInterval()
		previous := current
		current = current.Add(blockInterval)
		got, err := minter.CalculateBlockProvision(current, previous)
		require.NoError(t, err)
		totalBlockProvisions = totalBlockProvisions.Add(math.LegacyNewDecFromInt(got.Amount))
	}

	gotError := totalBlockProvisions.Sub(annualProvisions).Abs().Quo(annualProvisions)
	wantError := math.LegacyNewDecWithPrec(1, 2) // .01
	assert.True(t, gotError.LTE(wantError))
}

func randomBlockInterval() time.Duration {
	min := (14 * time.Second).Nanoseconds()
	max := (16 * time.Second).Nanoseconds()
	return time.Duration(randInRange(min, max))
}

// randInRange returns a random number in the range (min, max) inclusive.
func randInRange(min int64, max int64) int64 {
	return rand.Int63n(max-min) + min
}

func BenchmarkCalculateBlockProvision(b *testing.B) {
	b.ReportAllocs()
	minter := types.DefaultMinter()

	s1 := rand.NewSource(100)
	r1 := rand.New(s1)
	minter.AnnualProvisions = math.LegacyNewDec(r1.Int63n(1000000))
	current := time.Unix(r1.Int63n(1000000), 0)
	previous := current.Add(-time.Second * 15)

	for n := 0; n < b.N; n++ {
		_, err := minter.CalculateBlockProvision(current, previous)
		require.NoError(b, err)
	}
}

func BenchmarkCalculateInflationRate(b *testing.B) {
	b.ReportAllocs()
	minter := types.DefaultMinter()
	genesisTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	for n := 0; n < b.N; n++ {
		ctx := sdk.NewContext(nil, tmproto.Header{Height: int64(n)}, false, nil)
		minter.CalculateInflationRate(ctx, genesisTime)
	}
}

func Test_yearsSinceGenesis(t *testing.T) {
	type testCase struct {
		name    string
		current time.Time
		want    int64
	}

	genesis := time.Date(2023, 1, 1, 12, 30, 15, 0, time.UTC) // 2023-01-01T12:30:15Z
	oneDay, err := time.ParseDuration("24h")
	assert.NoError(t, err)
	oneWeek := oneDay * 7
	oneMonth := oneDay * 30
	oneYear := time.Duration(types.NanosecondsPerYear)
	twoYears := 2 * oneYear
	tenYears := 10 * oneYear
	tenYearsOneMonth := tenYears + oneMonth

	testCases := []testCase{
		{
			name:    "one day after genesis",
			current: genesis.Add(oneDay),
			want:    0,
		},
		{
			name:    "one day before genesis",
			current: genesis.Add(-oneDay),
			want:    0,
		},
		{
			name:    "one week after genesis",
			current: genesis.Add(oneWeek),
			want:    0,
		},
		{
			name:    "one month after genesis",
			current: genesis.Add(oneMonth),
			want:    0,
		},
		{
			name:    "one year after genesis",
			current: genesis.Add(oneYear),
			want:    1,
		},
		{
			name:    "two years after genesis",
			current: genesis.Add(twoYears),
			want:    2,
		},
		{
			name:    "ten years after genesis",
			current: genesis.Add(tenYears),
			want:    10,
		},
		{
			name:    "ten years and one month after genesis",
			current: genesis.Add(tenYearsOneMonth),
			want:    10,
		},
	}

	for _, tc := range testCases {
		got := types.YearsSinceGenesis(genesis, tc.current)
		assert.Equal(t, tc.want, got, tc.name)
	}
}

func TestMinterValidate(t *testing.T) {
	validTime := time.Now().UTC().Add(-time.Hour)
	testCases := []struct {
		name    string
		minter  types.Minter
		wantErr string
	}{
		{
			name: "valid minter",
			minter: types.Minter{
				InflationRate:     math.LegacyMustNewDecFromStr("0.05"),
				AnnualProvisions:  math.LegacyMustNewDecFromStr("1000000"),
				BondDenom:         types.DefaultBondDenom,
				PreviousBlockTime: &validTime,
			},
		},
		{
			name: "negative inflation rate",
			minter: types.Minter{
				InflationRate:    math.LegacyMustNewDecFromStr("-0.01"),
				AnnualProvisions: math.LegacyMustNewDecFromStr("1000000"),
				BondDenom:        types.DefaultBondDenom,
			},
			wantErr: "inflation rate -0.010000000000000000 should be positive",
		},
		{
			name: "negative annual provisions",
			minter: types.Minter{
				InflationRate:    math.LegacyMustNewDecFromStr("0.01"),
				AnnualProvisions: math.LegacyMustNewDecFromStr("-1000"),
				BondDenom:        types.DefaultBondDenom,
			},
			wantErr: "annual provisions -1000.000000000000000000 should be positive",
		},
		{
			name: "empty bond denom",
			minter: types.Minter{
				InflationRate:    math.LegacyMustNewDecFromStr("0.01"),
				AnnualProvisions: math.LegacyMustNewDecFromStr("1000"),
				BondDenom:        "  ",
			},
			wantErr: "bond denom cannot be empty",
		},
		{
			name: "bond denom with whitespace",
			minter: types.Minter{
				InflationRate:    math.LegacyMustNewDecFromStr("0.01"),
				AnnualProvisions: math.LegacyMustNewDecFromStr("1000"),
				BondDenom:        "ub bn",
			},
			wantErr: "bond denom cannot contain whitespace",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.minter.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
