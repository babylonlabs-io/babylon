package testnet_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades/v3rc3/testnet"
	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
)

func TestIndexFinalityContracts(t *testing.T) {
	babylonApp := app.Setup(t, false)
	bscKeeper := babylonApp.BTCStkConsumerKeeper
	ctx := babylonApp.NewContext(false)

	t.Run("empty_registry", func(t *testing.T) {
		err := testnet.IndexFinalityContracts(ctx, bscKeeper)
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
		err = testnet.IndexFinalityContracts(ctx, bscKeeper)
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
		err = testnet.IndexFinalityContracts(ctx, bscKeeper)
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
		err = testnet.IndexFinalityContracts(ctx, bscKeeper)
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
		err = testnet.IndexFinalityContracts(ctx, bscKeeper)
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
