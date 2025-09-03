package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestEmptyRewardGauge(t *testing.T) {
	emptyRewardGauge := &types.RewardGauge{
		Coins:          sdk.NewCoins(),
		WithdrawnCoins: sdk.NewCoins(),
	}
	rgBytes, err := emptyRewardGauge.Marshal()
	require.NoError(t, err)
	require.NotNil(t, rgBytes)         // the marshaled empty reward gauge is not nil
	require.True(t, len(rgBytes) == 0) // the marshalled empty reward gauge has 0 bytes
}

// StakeholderType enum for stakeholder type, used as key prefix in KVStore
type StakeholderType byte

const (
	FinalityProviderType StakeholderType = iota
	BTCDelegationType
)

func (st StakeholderType) Bytes() []byte {
	return []byte{byte(st)}
}

// Test to make sure the change of the Stakeholder type from iota to
// proto enum is non-breaking
func TestSetGetRewardGauge(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	k, ctx := keepertest.IncentiveKeeperWithStoreKey(t, storeKey, nil, nil, nil, nil)
	encCfg := appparams.DefaultEncodingConfig()

	storeService := runtime.NewKVStoreService(storeKey)
	store := storeService.OpenKVStore(ctx)
	storeAdaptor := runtime.KVStoreAdapter(store)
	rgStore := prefix.NewStore(storeAdaptor, types.RewardGaugeKey)

	addr1 := datagen.GenRandomAddress()
	addr2 := datagen.GenRandomAddress()
	g1 := datagen.GenRandomRewardGauge(r)
	g1.WithdrawnCoins = datagen.GenRandomCoins(r)
	g2 := datagen.GenRandomRewardGauge(r)
	g2.WithdrawnCoins = datagen.GenRandomCoins(r)

	// Set some values in the stakeholder types stores
	// using the previous type using iota
	fpStore := prefix.NewStore(rgStore, FinalityProviderType.Bytes())
	bdStore := prefix.NewStore(rgStore, BTCDelegationType.Bytes())

	g1Bz := encCfg.Codec.MustMarshal(g1)
	g2Bz := encCfg.Codec.MustMarshal(g2)
	fpStore.Set(addr1.Bytes(), g1Bz)
	bdStore.Set(addr2.Bytes(), g2Bz)

	// Retrieve values using the new enum type
	rg1 := k.GetRewardGauge(ctx, types.FINALITY_PROVIDER, addr1)
	rg2 := k.GetRewardGauge(ctx, types.BTC_STAKER, addr2)

	// Validate that both are equal
	require.Equal(t, g1, rg1)
	require.Equal(t, g2, rg2)
}
