package keeper

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func FuzzMigrationsMigrate1To2(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)

		numOfFps := datagen.GenRandomEpochNum(r) + 1
		expectedFps := make(map[string]types.FinalityProviderCurrentRewards, numOfFps)
		for fpNum := 0; fpNum < int(numOfFps); fpNum++ {
			fp := datagen.GenRandomAddress()

			fpCurrRwds := datagen.GenRandomFinalityProviderCurrentRewards(r)
			err := k.SetFinalityProviderCurrentRewards(ctx, fp, fpCurrRwds)
			require.NoError(t, err)

			expectedFps[fp.String()] = fpCurrRwds
		}

		// execute the migration
		m := NewMigrator(*k)
		err := m.Migrate1to2(ctx)
		require.NoError(t, err)

		err = k.IterateFpCurrentRewards(ctx, func(fp sdk.AccAddress, fpCurrRwds types.FinalityProviderCurrentRewards) error {
			fpAddr := fp.String()
			fpCurrRwdsInMap, ok := expectedFps[fpAddr]
			require.True(t, ok)

			// checks that the fp current rewards were multiplied by the rewards decimal
			fpCurrRwdsInMap.CurrentRewards = fpCurrRwdsInMap.CurrentRewards.MulInt(types.DecimalRewards)
			require.EqualValues(t, fpCurrRwdsInMap, fpCurrRwds)

			// deletes from the map to later check that there is no fp left
			delete(expectedFps, fpAddr)
			return nil
		})
		require.NoError(t, err)
		require.Len(t, expectedFps, 0, "should have pass all the fps in the created map")
	})
}
