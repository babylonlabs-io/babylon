package keeper

import (
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/testutil/addr"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/testutil/store"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
)

func FuzzCheckBTCDelegatorToFP(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
		del1, del2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		// only one set
		// del1 -> fp1
		k.setBTCDelegatorToFP(ctx, del1, fp1)
		count := 0
		err := k.iterBtcDelegationsByDelegator(ctx, del1, func(del, fp sdk.AccAddress) error {
			require.Equal(t, del.String(), del1.String())
			require.Equal(t, fp1.String(), fp.String())
			count++
			return nil
		})
		require.Equal(t, 1, count)
		require.NoError(t, err)

		// restart count every time
		// del1 -> fp1, fp2
		k.setBTCDelegatorToFP(ctx, del1, fp2)
		count = 0
		err = k.iterBtcDelegationsByDelegator(ctx, del1, func(del, fp sdk.AccAddress) error {
			count++
			require.Equal(t, del.String(), del1.String())
			if fp.Equals(fp1) {
				require.Equal(t, fp1.String(), fp.String())
				return nil
			}

			require.Equal(t, fp2.String(), fp.String())
			return nil
		})
		require.Equal(t, 2, count)
		require.NoError(t, err)

		// new delegator
		// del2 -> fp2
		k.setBTCDelegatorToFP(ctx, del2, fp2)
		count = 0
		err = k.iterBtcDelegationsByDelegator(ctx, del2, func(del, fp sdk.AccAddress) error {
			count++
			require.Equal(t, del.String(), del2.String())
			require.Equal(t, fp2.String(), fp.String())
			return nil
		})
		require.Equal(t, 1, count)
		require.NoError(t, err)

		// deletes del1 -> fp1
		// iterates again should only have the del1 -> fp2
		count = 0
		k.deleteBTCDelegatorToFP(ctx, del1, fp1)
		err = k.iterBtcDelegationsByDelegator(ctx, del1, func(del, fp sdk.AccAddress) error {
			require.Equal(t, del.String(), del1.String())
			require.Equal(t, fp2.String(), fp.String())
			count++
			return nil
		})
		require.Equal(t, 1, count)
		require.NoError(t, err)
	})
}

func FuzzCheckBTCDelegationRewardsTracker(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
		del1, del2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		// fp1, del1
		err := k.setBTCDelegationRewardsTracker(ctx, fp1, del1, types.NewBTCDelegationRewardsTracker(0, math.NewInt(100)))
		require.NoError(t, err)

		count := 0
		err = k.IterateBTCDelegationRewardsTracker(ctx, fp1, func(fp, del sdk.AccAddress) error {
			count++
			require.Equal(t, fp1, fp)
			require.Equal(t, del1, del)
			return nil
		})
		require.Equal(t, 1, count)
		require.NoError(t, err)

		// fp1, del2
		err = k.setBTCDelegationRewardsTracker(ctx, fp1, del2, types.NewBTCDelegationRewardsTracker(0, math.NewInt(100)))
		require.NoError(t, err)

		count = 0
		err = k.IterateBTCDelegationRewardsTracker(ctx, fp1, func(fp, del sdk.AccAddress) error {
			count++
			require.Equal(t, fp1, fp)
			if del1.Equals(del) {
				require.Equal(t, del1, del)
				return nil
			}
			require.Equal(t, del2, del)
			return nil
		})
		require.Equal(t, 2, count)
		require.NoError(t, err)

		// fp2, del1
		amtFp2Del1 := datagen.RandomMathInt(r, 20000)
		startPeriodFp2Del1 := datagen.RandomInt(r, 200)
		err = k.setBTCDelegationRewardsTracker(ctx, fp2, del1, types.NewBTCDelegationRewardsTracker(startPeriodFp2Del1, amtFp2Del1))
		require.NoError(t, err)

		btcDelRwdTracker, err := k.GetBTCDelegationRewardsTracker(ctx, fp2, del1)
		require.NoError(t, err)
		require.Equal(t, amtFp2Del1.String(), btcDelRwdTracker.TotalActiveSat.String())
		require.Equal(t, startPeriodFp2Del1, btcDelRwdTracker.StartPeriodCumulativeReward)

		count = 0
		err = k.IterateBTCDelegationRewardsTracker(ctx, fp2, func(fp, del sdk.AccAddress) error {
			count++
			require.Equal(t, fp2, fp)
			require.Equal(t, del1, del)
			return nil
		})
		require.Equal(t, 1, count)
		require.NoError(t, err)
	})
}

func NewKeeperWithCtx(t *testing.T) (Keeper, sdk.Context) {
	encConf := appparams.DefaultEncodingConfig()
	ctx, kvStore := store.NewStoreWithCtx(t, types.ModuleName)
	k := NewKeeper(encConf.Codec, kvStore, nil, nil, nil, addr.AccGov.String(), addr.AccFeeCollector.String())
	return k, ctx
}
