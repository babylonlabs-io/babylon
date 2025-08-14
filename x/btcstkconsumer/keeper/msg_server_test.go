package keeper_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"

	"github.com/babylonlabs-io/babylon/v3/app"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	wasmtest "github.com/babylonlabs-io/babylon/v3/wasmbinding/test"
	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/keeper"
	"github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
)

func setupMsgServer(t testing.TB) (keeper.Keeper, types.MsgServer, context.Context) {
	k, ctx := keepertest.BTCStkConsumerKeeper(t)
	return k, keeper.NewMsgServerImpl(k), ctx
}

func TestMsgServer(t *testing.T) {
	k, ms, ctx := setupMsgServer(t)
	require.NotNil(t, ms)
	require.NotNil(t, ctx)
	require.NotEmpty(t, k)
}

func TestRejectConsumerIdSameAsChainId(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	babylonApp, ctx := wasmtest.SetupAppWithContext(t)
	bscKeeper := babylonApp.BTCStkConsumerKeeper
	msgServer := keeper.NewMsgServerImpl(bscKeeper)

	ctx = ctx.WithChainID("babylon-1")
	/*
		Test registering Cosmos consumer
	*/
	// generate a random consumer register
	consumerRegister := datagen.GenRandomCosmosConsumerRegisterWithoutChannelSet(r)
	// mock IBC light client
	babylonApp.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerRegister.ConsumerId, &ibctmtypes.ClientState{})
	// Register the consumer
	_, err := msgServer.RegisterConsumer(ctx, &types.MsgRegisterConsumer{
		ConsumerId:               ctx.ChainID(),
		ConsumerName:             consumerRegister.ConsumerName,
		ConsumerDescription:      consumerRegister.ConsumerDescription,
		BabylonRewardsCommission: consumerRegister.BabylonRewardsCommission,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidConsumerIDs)
}

func FuzzRegisterConsumer(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		babylonApp, ctx := wasmtest.SetupAppWithContext(t)
		bscKeeper := babylonApp.BTCStkConsumerKeeper
		msgServer := keeper.NewMsgServerImpl(bscKeeper)

		/*
			Test gov-gated registration
		*/
		// enable gov-gated registration
		err := bscKeeper.SetParams(ctx, types.Params{
			PermissionedIntegration: true,
		})
		require.NoError(t, err)
		// generate a random consumer register
		consumerRegister := datagen.GenRandomCosmosConsumerRegisterWithoutChannelSet(r)
		// Register the consumer
		_, err = msgServer.RegisterConsumer(ctx, &types.MsgRegisterConsumer{
			ConsumerId:               consumerRegister.ConsumerId,
			ConsumerName:             consumerRegister.ConsumerName,
			ConsumerDescription:      consumerRegister.ConsumerDescription,
			BabylonRewardsCommission: consumerRegister.BabylonRewardsCommission,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, govtypes.ErrInvalidSigner)
		// disable gov-gated registration
		err = bscKeeper.SetParams(ctx, types.Params{
			PermissionedIntegration: false,
		})
		require.NoError(t, err)

		/*
			Test registering Cosmos consumer
		*/
		// generate a random consumer register
		consumerRegister = datagen.GenRandomCosmosConsumerRegisterWithoutChannelSet(r)
		// mock IBC light client
		babylonApp.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerRegister.ConsumerId, &ibctmtypes.ClientState{})
		// Register the consumer
		_, err = msgServer.RegisterConsumer(ctx, &types.MsgRegisterConsumer{
			ConsumerId:               consumerRegister.ConsumerId,
			ConsumerName:             consumerRegister.ConsumerName,
			ConsumerDescription:      consumerRegister.ConsumerDescription,
			BabylonRewardsCommission: consumerRegister.BabylonRewardsCommission,
		})
		require.NoError(t, err)
		// check that the consumer is registered
		consumerRegister2, err := bscKeeper.GetConsumerRegister(ctx, consumerRegister.ConsumerId)
		require.NoError(t, err)
		require.Equal(t, consumerRegister.String(), consumerRegister2.String())

		/*
			Test registering rollup consumer
		*/
		// mock the wasm contract
		contractAddr := mockSmartContract(t, ctx, babylonApp)
		// generate a random consumer register
		consumerRegister = datagen.GenRandomRollupRegister(r, contractAddr.String())
		// Register the consumer
		_, err = msgServer.RegisterConsumer(ctx, &types.MsgRegisterConsumer{
			ConsumerId:                    consumerRegister.ConsumerId,
			ConsumerName:                  consumerRegister.ConsumerName,
			ConsumerDescription:           consumerRegister.ConsumerDescription,
			RollupFinalityContractAddress: contractAddr.String(),
			BabylonRewardsCommission:      consumerRegister.BabylonRewardsCommission,
		})
		require.NoError(t, err)
		// check that the consumer is registered
		consumerRegister2, err = bscKeeper.GetConsumerRegister(ctx, consumerRegister.ConsumerId)
		require.NoError(t, err)
		require.Equal(t, consumerRegister.String(), consumerRegister2.String())
		require.Equal(t, types.ConsumerType_ROLLUP, consumerRegister2.Type())
	})
}

func mockSmartContract(t *testing.T, ctx sdk.Context, babylonApp *app.BabylonApp) sdk.AccAddress {
	return wasmtest.DeployTestContract(t, ctx, babylonApp, sdk.AccAddress{}, "../../../wasmbinding/testdata/artifacts/testdata.wasm")
}
