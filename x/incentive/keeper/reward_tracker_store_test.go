package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
)

func FuzzCheckSetBTCDelegatorToFP(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		k, ctx := NewKeeperWithCtx(t)
		fp1, fp2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()
		del1, del2 := datagen.GenRandomAddress(), datagen.GenRandomAddress()

		// only one set
		// del1 -> fp1
		k.setBTCDelegatorToFP(ctx, del1, fp1)
		count := 0
		err := k.iterBtcDelegatorToFP(ctx, del1, func(del, fp sdk.AccAddress) error {
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
		err = k.iterBtcDelegatorToFP(ctx, del1, func(del, fp sdk.AccAddress) error {
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
		err = k.iterBtcDelegatorToFP(ctx, del2, func(del, fp sdk.AccAddress) error {
			count++
			require.Equal(t, del.String(), del2.String())
			require.Equal(t, fp2.String(), fp.String())
			return nil
		})
		require.Equal(t, 1, count)
		require.NoError(t, err)
	})
}

func NewKeeperWithCtx(t *testing.T) (Keeper, sdk.Context) {
	encConf := appparams.DefaultEncodingConfig()

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	govAcc := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	feeColl := authtypes.NewModuleAddress(authtypes.FeeCollectorName).String()
	k := NewKeeper(encConf.Codec, runtime.NewKVStoreService(storeKey), nil, nil, nil, govAcc, feeColl)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	ctx = ctx.WithHeaderInfo(header.Info{})

	return k, ctx
}
