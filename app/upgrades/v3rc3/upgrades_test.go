package v3rc3_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/header"
	"cosmossdk.io/math"
	"cosmossdk.io/x/upgrade"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/stretchr/testify/suite"

	upgradetypes "cosmossdk.io/x/upgrade/types"
<<<<<<< HEAD
	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
	"github.com/babylonlabs-io/babylon/v3/app/upgrades"
	v3rc3 "github.com/babylonlabs-io/babylon/v3/app/upgrades/v3rc3"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	btcstktypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
=======
	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/app/signingcontext"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	v3rc3 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v3rc3"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	btcstktypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
>>>>>>> d79f7c56 (imp(btcstkconsumer): add finality contract idx (#1596))
	"github.com/stretchr/testify/require"
)

const (
	DummyUpgradeHeight = 5
)

func TestGetLargestBtcReorg(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	headerInfo1 := datagen.GenRandomBTCHeaderInfo(r)
	headerInfo1.Height = 100

	headerInfo2 := datagen.GenRandomBTCHeaderInfo(r)
	headerInfo2.Height = 200

	tcs := []struct {
		name               string
		largestBtcReorg    btcstktypes.LargestBtcReOrg
		oldLargestBtcReorg btcstktypes.LargestBtcReOrg
		err                error
		errOld             error
		expectedResult     *btcstktypes.LargestBtcReOrg
	}{
		{
			name: "both valid - choose largest diff",
			largestBtcReorg: btcstktypes.LargestBtcReOrg{
				BlockDiff:    10,
				RollbackFrom: headerInfo1,
				RollbackTo:   headerInfo2,
			},
			oldLargestBtcReorg: btcstktypes.LargestBtcReOrg{
				BlockDiff:    20,
				RollbackFrom: headerInfo2,
				RollbackTo:   headerInfo1,
			},
			err:    nil,
			errOld: nil,
			expectedResult: &btcstktypes.LargestBtcReOrg{
				BlockDiff:    20,
				RollbackFrom: headerInfo2,
				RollbackTo:   headerInfo1,
			},
		},
		{
			name: "both valid - current is larger",
			largestBtcReorg: btcstktypes.LargestBtcReOrg{
				BlockDiff:    30,
				RollbackFrom: headerInfo1,
				RollbackTo:   headerInfo2,
			},
			oldLargestBtcReorg: btcstktypes.LargestBtcReOrg{
				BlockDiff:    20,
				RollbackFrom: headerInfo2,
				RollbackTo:   headerInfo1,
			},
			err:    nil,
			errOld: nil,
			expectedResult: &btcstktypes.LargestBtcReOrg{
				BlockDiff:    30,
				RollbackFrom: headerInfo1,
				RollbackTo:   headerInfo2,
			},
		},
		{
			name: "only current valid",
			largestBtcReorg: btcstktypes.LargestBtcReOrg{
				BlockDiff:    15,
				RollbackFrom: headerInfo1,
				RollbackTo:   headerInfo2,
			},
			oldLargestBtcReorg: btcstktypes.LargestBtcReOrg{},
			err:                nil,
			errOld:             collections.ErrNotFound,
			expectedResult: &btcstktypes.LargestBtcReOrg{
				BlockDiff:    15,
				RollbackFrom: headerInfo1,
				RollbackTo:   headerInfo2,
			},
		},
		{
			name:            "only old valid",
			largestBtcReorg: btcstktypes.LargestBtcReOrg{},
			oldLargestBtcReorg: btcstktypes.LargestBtcReOrg{
				BlockDiff:    25,
				RollbackFrom: headerInfo2,
				RollbackTo:   headerInfo1,
			},
			err:    collections.ErrNotFound,
			errOld: nil,
			expectedResult: &btcstktypes.LargestBtcReOrg{
				BlockDiff:    25,
				RollbackFrom: headerInfo2,
				RollbackTo:   headerInfo1,
			},
		},
		{
			name:               "neither valid",
			largestBtcReorg:    btcstktypes.LargestBtcReOrg{},
			oldLargestBtcReorg: btcstktypes.LargestBtcReOrg{},
			err:                collections.ErrNotFound,
			errOld:             collections.ErrNotFound,
			expectedResult:     nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			actual := v3rc3.GetLargestBtcReorg(tc.largestBtcReorg, tc.oldLargestBtcReorg, tc.err, tc.errOld)
			if tc.expectedResult == nil {
				require.Nil(t, actual)
				return
			}
			require.NotNil(t, actual)
			require.Equal(t, tc.expectedResult.BlockDiff, actual.BlockDiff)
			require.Equal(t, tc.expectedResult.RollbackFrom, actual.RollbackFrom)
			require.Equal(t, tc.expectedResult.RollbackTo, actual.RollbackTo)
		})
	}
}

func TestIndexFinalityContracts(t *testing.T) {
	babylonApp := app.Setup(t, false)
	bscKeeper := babylonApp.BTCStkConsumerKeeper
	ctx := babylonApp.NewContext(false)

	t.Run("empty_registry", func(t *testing.T) {
		err := v3rc3.IndexFinalityContracts(ctx, bscKeeper)
		require.NoError(t, err, "IndexFinalityContracts should succeed with empty registry")
	})

	t.Run("rollup_consumer_with_finality_contract", func(t *testing.T) {
		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		contractAddr := "0x1234567890abcdef1234567890abcdef12345678"

		rollupConsumer := types.NewRollupConsumerRegister(
			"test-rollup-1",
			"Test Rollup Consumer",
			"Test rollup description",
			contractAddr,
			math.LegacyNewDecWithPrec(5, 2),
		)

		// Directly insert consumer without triggering contract indexing (simulating pre-upgrade state)
		err := bscKeeper.ConsumerRegistry.Set(ctx, rollupConsumer.ConsumerId, *rollupConsumer)
		require.NoError(t, err)

		// Verify contract is not indexed yet (pre-upgrade state)
		isRegistered, err := bscKeeper.IsFinalityContractRegistered(ctx, contractAddr)
		require.NoError(t, err)
		require.False(t, isRegistered, "Contract should not be indexed yet (pre-upgrade state)")

		// Run upgrade function
		err = v3rc3.IndexFinalityContracts(ctx, bscKeeper)
		require.NoError(t, err)

		// Verify contract is now indexed after upgrade
		isRegistered, err = bscKeeper.IsFinalityContractRegistered(ctx, contractAddr)
		require.NoError(t, err)
		require.True(t, isRegistered, "Contract should be indexed after upgrade")
	})

	t.Run("rollup_consumer_without_finality_contract", func(t *testing.T) {
		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		rollupConsumer := types.NewRollupConsumerRegister(
			"test-rollup-2",
			"Test Rollup Consumer 2",
			"Test rollup description 2",
			"",
			math.LegacyNewDecWithPrec(10, 2),
		)

		// Directly insert consumer without triggering contract indexing (simulating pre-upgrade state)
		err := bscKeeper.ConsumerRegistry.Set(ctx, rollupConsumer.ConsumerId, *rollupConsumer)
		require.NoError(t, err)

		// Run upgrade function - should succeed even with empty contract address
		err = v3rc3.IndexFinalityContracts(ctx, bscKeeper)
		require.NoError(t, err, "Should succeed even with empty contract address")
	})

	t.Run("cosmos_consumer_ignored", func(t *testing.T) {
		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		cosmosConsumer := types.NewCosmosConsumerRegister(
			"test-cosmos-1",
			"Test Cosmos Consumer",
			"Test cosmos description",
			math.LegacyNewDecWithPrec(15, 2),
		)

		// Directly insert consumer (simulating pre-upgrade state)
		err := bscKeeper.ConsumerRegistry.Set(ctx, cosmosConsumer.ConsumerId, *cosmosConsumer)
		require.NoError(t, err)

		// Run upgrade function - should succeed and ignore cosmos consumers
		err = v3rc3.IndexFinalityContracts(ctx, bscKeeper)
		require.NoError(t, err, "Should succeed and ignore cosmos consumers")
	})

	t.Run("multiple_rollup_consumers", func(t *testing.T) {
		babylonApp := app.Setup(t, false)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		ctx := babylonApp.NewContext(false)

		contractAddr1 := "0x1111111111111111111111111111111111111111"
		contractAddr2 := "0x2222222222222222222222222222222222222222"

		rollup1 := types.NewRollupConsumerRegister(
			"rollup-1",
			"Rollup 1",
			"Description 1",
			contractAddr1,
			math.LegacyNewDecWithPrec(5, 2),
		)

		rollup2 := types.NewRollupConsumerRegister(
			"rollup-2",
			"Rollup 2",
			"Description 2",
			contractAddr2,
			math.LegacyNewDecWithPrec(10, 2),
		)

		rollupEmpty := types.NewRollupConsumerRegister(
			"rollup-empty",
			"Rollup Empty",
			"Description Empty",
			"",
			math.LegacyNewDecWithPrec(15, 2),
		)

		// Directly insert consumers without triggering contract indexing (simulating pre-upgrade state)
		err := bscKeeper.ConsumerRegistry.Set(ctx, rollup1.ConsumerId, *rollup1)
		require.NoError(t, err)
		err = bscKeeper.ConsumerRegistry.Set(ctx, rollup2.ConsumerId, *rollup2)
		require.NoError(t, err)
		err = bscKeeper.ConsumerRegistry.Set(ctx, rollupEmpty.ConsumerId, *rollupEmpty)
		require.NoError(t, err)

		// Verify contracts are not indexed yet (pre-upgrade state)
		isRegistered1, err := bscKeeper.IsFinalityContractRegistered(ctx, contractAddr1)
		require.NoError(t, err)
		require.False(t, isRegistered1, "Contract 1 should not be indexed yet (pre-upgrade)")

		isRegistered2, err := bscKeeper.IsFinalityContractRegistered(ctx, contractAddr2)
		require.NoError(t, err)
		require.False(t, isRegistered2, "Contract 2 should not be indexed yet (pre-upgrade)")

		// Run upgrade function
		err = v3rc3.IndexFinalityContracts(ctx, bscKeeper)
		require.NoError(t, err)

		// Verify contracts are now indexed after upgrade
		isRegistered1, err = bscKeeper.IsFinalityContractRegistered(ctx, contractAddr1)
		require.NoError(t, err)
		require.True(t, isRegistered1, "Contract 1 should be indexed after upgrade")

		isRegistered2, err = bscKeeper.IsFinalityContractRegistered(ctx, contractAddr2)
		require.NoError(t, err)
		require.True(t, isRegistered2, "Contract 2 should be indexed after upgrade")
	})
}

type UpgradeTestSuite struct {
	suite.Suite

	ctx       sdk.Context
	app       *app.BabylonApp
	preModule appmodule.HasPreBlocker

	fpBtcPkToDelete *bbn.BIP340PubKey
	largestBtcReorg *btcstktypes.LargestBtcReOrg
}

func TestUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (s *UpgradeTestSuite) TestUpgrade() {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	tcs := []struct {
		msg         string
		preUpgrade  func()
		upgrade     func()
		postUpgrade func()
	}{
		{
			"Test upgrade v3rc3 with duplicated fp addr and largest btc reorg in prefix 13",
			func() {
				s.PreUpgrade()

				btcStkK, ctx := s.app.BTCStakingKeeper, s.ctx
				sigCtx := signingcontext.FpPopContextV0(ctx.ChainID(), btcStkK.ModuleAddress())

				msgCreateFp, err := datagen.GenRandomMsgCreateFinalityProvider(r, sigCtx)
				s.NoError(err)
				err = btcStkK.AddFinalityProvider(ctx, msgCreateFp)
				s.NoError(err)

				s.fpBtcPkToDelete = msgCreateFp.BtcPk
				fp, err := btcStkK.GetFinalityProvider(ctx, *msgCreateFp.BtcPk)
				s.NoError(err)

				// creates another fp with an different btc fp pk, but same babylon addresss
				// and some vote
				btcSK, _, err := datagen.GenRandomBTCKeyPair(r)
				s.NoError(err)
				btcPK := btcSK.PubKey()
				bip340PK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)

				pop, err := datagen.NewPoPBTC(sigCtx, sdk.MustAccAddressFromBech32(msgCreateFp.Addr), btcSK)
				s.NoError(err)

				fp.BtcPk = bip340PK
				fp.Pop = pop
				fp.HighestVotedHeight += 2
				btcStkK.SetFinalityProvider(ctx, fp)

				// set largest btc reorg in prefix 13
				btcStkStoreKey := s.app.GetKey(btcstktypes.StoreKey)
				btcStkStoreService := runtime.NewKVStoreService(btcStkStoreKey)
				sb := collections.NewSchemaBuilder(btcStkStoreService)
				oldLargestBtcReorgItem := collections.NewItem(
					sb,
					v3rc3.OldTestnetLargestBtcReorgInBlocks,
					"largest_btc_reorg",
					codec.CollValue[btcstktypes.LargestBtcReOrg](app.GetEncodingConfig().Codec),
				)

				from, to := datagen.GenRandomBTCHeaderInfo(r), datagen.GenRandomBTCHeaderInfo(r)
				largestReorg := btcstktypes.NewLargestBtcReOrg(from, to)

				s.largestBtcReorg = &largestReorg
				err = oldLargestBtcReorgItem.Set(ctx, largestReorg)
				s.NoError(err)

				err = btcStkK.LargestBtcReorg.Remove(ctx)
				s.NoError(err)
			},
			s.Upgrade,
			func() {
				s.PostUpgrade()
				btcStkK, ctx := s.app.BTCStakingKeeper, s.ctx

				// check that fp was deleted
				isDeleted := btcStkK.IsFinalityProviderDeleted(ctx, s.fpBtcPkToDelete)
				s.True(isDeleted)

				err := btcStkK.IterateFinalityProvider(ctx, func(fp btcstktypes.FinalityProvider) error {
					if fp.BtcPk.Equals(s.fpBtcPkToDelete) {
						isDeleted := btcStkK.IsFinalityProviderDeleted(ctx, fp.BtcPk)
						s.True(isDeleted)
						return nil
					}
					isDeleted := btcStkK.IsFinalityProviderDeleted(ctx, fp.BtcPk)
					s.False(isDeleted)
					return nil
				})
				s.NoError(err)

				largestBtcReorg := btcStkK.GetLargestBtcReorg(ctx)
				s.EqualValues(s.largestBtcReorg, largestBtcReorg)
			},
		},
		{
			"Test upgrade v3rc3 witout duplicated fp addr and without largest btc reorg in any prefix",
			func() {
				s.PreUpgrade()

				btcStkK, ctx := s.app.BTCStakingKeeper, s.ctx
				sigCtx := signingcontext.FpPopContextV0(ctx.ChainID(), btcStkK.ModuleAddress())

				msgCreateFp, err := datagen.GenRandomMsgCreateFinalityProvider(r, sigCtx)
				s.NoError(err)
				err = btcStkK.AddFinalityProvider(ctx, msgCreateFp)
				s.NoError(err)

				msgCreateFp2, err := datagen.GenRandomMsgCreateFinalityProvider(r, sigCtx)
				s.NoError(err)
				err = btcStkK.AddFinalityProvider(ctx, msgCreateFp2)
				s.NoError(err)

				// make sure both prefix have nothing
				btcStkStoreKey := s.app.GetKey(btcstktypes.StoreKey)
				btcStkStoreService := runtime.NewKVStoreService(btcStkStoreKey)
				sb := collections.NewSchemaBuilder(btcStkStoreService)
				oldLargestBtcReorgItem := collections.NewItem(
					sb,
					v3rc3.OldTestnetLargestBtcReorgInBlocks,
					"largest_btc_reorg",
					codec.CollValue[btcstktypes.LargestBtcReOrg](app.GetEncodingConfig().Codec),
				)

				err = oldLargestBtcReorgItem.Remove(ctx)
				s.NoError(err)

				err = btcStkK.LargestBtcReorg.Remove(ctx)
				s.NoError(err)
			},
			s.Upgrade,
			func() {
				s.PostUpgrade()
				btcStkK, ctx := s.app.BTCStakingKeeper, s.ctx

				err := btcStkK.IterateFinalityProvider(ctx, func(fp btcstktypes.FinalityProvider) error {
					isDeleted := btcStkK.IsFinalityProviderDeleted(ctx, fp.BtcPk)
					s.False(isDeleted)
					return nil
				})
				s.NoError(err)

				largestBtcReorg := btcStkK.GetLargestBtcReorg(ctx)
				s.Nil(largestBtcReorg)
			},
		},
	}

	for _, tc := range tcs {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			s.SetupTest() // reset

			tc.preUpgrade()
			tc.upgrade()
			tc.postUpgrade()
		})
	}
}

func (s *UpgradeTestSuite) SetupTest() {
	// add the upgrade plan
	app.Upgrades = []upgrades.Upgrade{v3rc3.Upgrade}

	// set up app
	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())
}

func (s *UpgradeTestSuite) PreUpgrade() {}

func (s *UpgradeTestSuite) Upgrade() {
	// inject upgrade plan
	s.ctx = s.ctx.WithBlockHeight(DummyUpgradeHeight - 1)
	plan := upgradetypes.Plan{Name: v3rc3.UpgradeName, Height: DummyUpgradeHeight}
	err := s.app.UpgradeKeeper.ScheduleUpgrade(s.ctx, plan)
	s.NoError(err)

	// ensure upgrade plan exists
	actualPlan, err := s.app.UpgradeKeeper.GetUpgradePlan(s.ctx)
	s.NoError(err)
	s.Equal(plan, actualPlan)

	// execute upgrade
	s.ctx = s.ctx.WithHeaderInfo(header.Info{Height: DummyUpgradeHeight, Time: s.ctx.BlockTime().Add(time.Second)}).WithBlockHeight(DummyUpgradeHeight)
	s.NotPanics(func() {
		_, err := s.preModule.PreBlock(s.ctx)
		s.Require().NoError(err)
	})
}

func (s *UpgradeTestSuite) PostUpgrade() {

}
