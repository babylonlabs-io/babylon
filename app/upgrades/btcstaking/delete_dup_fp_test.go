package btcstaking_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

<<<<<<< HEAD:app/upgrades/delete_dup_fp_test.go
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btcstktypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
=======
	_ "github.com/babylonlabs-io/babylon/v4/app/params" // import this to run the init func that sets the address prefixes
	"github.com/babylonlabs-io/babylon/v4/app/upgrades/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
>>>>>>> d79f7c56 (imp(btcstkconsumer): add finality contract idx (#1596)):app/upgrades/btcstaking/delete_dup_fp_test.go
)

type FpFromTestData struct {
	Addr               string            `json:"addr,omitempty"`
	BtcPk              *bbn.BIP340PubKey `json:"btc_pk,omitempty"`
	HighestVotedHeight uint32            `json:"highest_voted_height,omitempty"`
}

type MockBtcStkKeeper struct {
	mock.Mock
	fps              []btcstktypes.FinalityProvider
	setBbnAddrsCount uint64
	deletedFps       []*bbn.BIP340PubKey
	totalSatsStaked  map[string]uint64
}

func (m *MockBtcStkKeeper) IterateFinalityProvider(ctx context.Context, f func(fp btcstktypes.FinalityProvider) error) error {
	for _, fp := range m.fps {
		if err := f(fp); err != nil {
			return err
		}
	}
	return nil
}

func (m *MockBtcStkKeeper) SetFpBbnAddr(ctx context.Context, fpAddr sdk.AccAddress) error {
	m.setBbnAddrsCount++
	return nil
}

func (m *MockBtcStkKeeper) SoftDeleteFinalityProvider(ctx context.Context, fpBtcPk *bbn.BIP340PubKey) error {
	m.deletedFps = append(m.deletedFps, fpBtcPk)
	return nil
}

func (m *MockBtcStkKeeper) FpTotalSatsStaked(ctx context.Context, fpBTCPK *bbn.BIP340PubKey) (uint64, error) {
	if sats, exists := m.totalSatsStaked[fpBTCPK.MarshalHex()]; exists {
		return sats, nil
	}
	return 0, nil
}

func TestFpSoftDeleteDupAddr(t *testing.T) {
	testCases := []struct {
		name                  string
		filename              string
		expectedDeletedBtcPks []string
		totalSatsStaked       map[string]uint64
	}{
		{
			name:     "mainnet data",
			filename: "out-fps-mainnet.json",
			expectedDeletedBtcPks: []string{
				"ecfeb762082dfb7cc073abe0aa0c656c47bd7ed60486a7353000600341110378",
				"a2c2e0cbb10932a3de4ad7c2001e1c51993ea673762fd34196d4fd9e6042b743",
			},
			totalSatsStaked: make(map[string]uint64),
		},
		{
			name:     "testnet data",
			filename: "out-fps-testnet.json",
			expectedDeletedBtcPks: []string{
				"c8ff61aafd02b97a2d0490d52ae324157987af558d3974e1bf399cfefac5884c",
				"5f30400bc58614d5e182ebe889f30d61e3c78ab78f9e7782d393db1e867cbcff",
				"feb1fdfe5f2e844f86aa722182d7ab1f0062d72ff403c7e38994f0c0fdbaf5ee",
				"fa4809bbe416627fc6c103de9541ad9605125685e38b181ff846612ffa761b0e",
				"5949b0c8e57dcb338471aec80141f7e6b73125779446672fcc3f600ccf46674d",
				"9359fb4d5a2f22c1b927d21fd05d23837b67e4e3e83ddd559ad99d1f2f8493e5",
			},
			totalSatsStaked: map[string]uint64{
				"9359fb4d5a2f22c1b927d21fd05d23837b67e4e3e83ddd559ad99d1f2f8493e5": 0,
				"d4085d4f3f4b44f32b05dd86bb1ca5cc2e9e0c2889cc7bf84618445c4dd9a239": 55400,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testDataPath := filepath.Join("testdata", tc.filename)
			data, err := os.ReadFile(testDataPath)
			require.NoError(t, err, "failed to read test data file %s", tc.filename)

			// loads the data from the file without PoP mess
			var fpsFromFile []FpFromTestData
			err = json.Unmarshal(data, &fpsFromFile)
			require.NoError(t, err, "failed to unmarshal finality providers from %s", tc.filename)

			fps := make([]btcstktypes.FinalityProvider, len(fpsFromFile))
			for i, fpFromFile := range fpsFromFile {
				// adds important part for filtering
				fps[i] = btcstktypes.FinalityProvider{
					Addr:               fpFromFile.Addr,
					BtcPk:              fpFromFile.BtcPk,
					HighestVotedHeight: fpFromFile.HighestVotedHeight,
				}
			}

			mockBtcStkKeeper := &MockBtcStkKeeper{
				fps:             fps,
				totalSatsStaked: tc.totalSatsStaked,
			}

			ctx := context.Background()
			err = btcstaking.FpSoftDeleteDupAddr(ctx, mockBtcStkKeeper)
			require.NoError(t, err)
			// Verify that SetFpBbnAddr was called for each FP
			require.EqualValues(t, mockBtcStkKeeper.setBbnAddrsCount, len(fps), "SetFpBbnAddr should be called for each FP")
			// Verify the correct number of FPs were marked for deletion
			require.Len(t, mockBtcStkKeeper.deletedFps, len(tc.expectedDeletedBtcPks), "expected %d BTC public keys to be soft deleted for %s", len(tc.expectedDeletedBtcPks), tc.name)

			// Convert deleted BTC public keys to hex strings
			actualDeletedBtcPkHex := make([]string, len(mockBtcStkKeeper.deletedFps))
			for i, btcPk := range mockBtcStkKeeper.deletedFps {
				actualDeletedBtcPkHex[i] = btcPk.MarshalHex()
			}
			require.ElementsMatch(t, tc.expectedDeletedBtcPks, actualDeletedBtcPkHex, "Deleted BTC public keys should match expected values for %s", tc.name)
		})
	}
}
