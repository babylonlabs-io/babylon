package keeper_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	wasmtest "github.com/babylonlabs-io/babylon/wasmbinding/test"
	"github.com/babylonlabs-io/babylon/x/btcstkconsumer/keeper"
	"github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	ibctmtypes "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
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
		consumerRegister := datagen.GenRandomCosmosConsumerRegister(r)
		// Register the consumer
		_, err = msgServer.RegisterConsumer(ctx, &types.MsgRegisterConsumer{
			ConsumerId:          consumerRegister.ConsumerId,
			ConsumerName:        consumerRegister.ConsumerName,
			ConsumerDescription: consumerRegister.ConsumerDescription,
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
		consumerRegister = datagen.GenRandomCosmosConsumerRegister(r)
		// mock IBC light client
		babylonApp.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerRegister.ConsumerId, &ibctmtypes.ClientState{})
		// Register the consumer
		_, err = msgServer.RegisterConsumer(ctx, &types.MsgRegisterConsumer{
			ConsumerId:          consumerRegister.ConsumerId,
			ConsumerName:        consumerRegister.ConsumerName,
			ConsumerDescription: consumerRegister.ConsumerDescription,
		})
		require.NoError(t, err)
		// check that the consumer is registered
		consumerRegister2, err := bscKeeper.GetConsumerRegister(ctx, consumerRegister.ConsumerId)
		require.NoError(t, err)
		require.Equal(t, consumerRegister.String(), consumerRegister2.String())

		/*
			Test registering ETH L2 consumer
		*/
		// mock the wasm contract
		contractAddr := mockSmartContract(t, ctx, babylonApp)
		// generate a random consumer register
		consumerRegister = datagen.GenRandomETHL2Register(r, contractAddr.String())
		// Register the consumer
		_, err = msgServer.RegisterConsumer(ctx, &types.MsgRegisterConsumer{
			ConsumerId:                   consumerRegister.ConsumerId,
			ConsumerName:                 consumerRegister.ConsumerName,
			ConsumerDescription:          consumerRegister.ConsumerDescription,
			EthL2FinalityContractAddress: contractAddr.String(),
		})
		require.NoError(t, err)
		// check that the consumer is registered
		consumerRegister2, err = bscKeeper.GetConsumerRegister(ctx, consumerRegister.ConsumerId)
		require.NoError(t, err)
		require.Equal(t, consumerRegister.String(), consumerRegister2.String())
		require.Equal(t, types.ConsumerType_ETH_L2, consumerRegister2.Type())
	})
}

func mockSmartContract(t *testing.T, ctx sdk.Context, babylonApp *app.BabylonApp) sdk.AccAddress {
	return wasmtest.DeployTestContract(t, ctx, babylonApp, sdk.AccAddress{}, "../../../wasmbinding/testdata/artifacts/testdata.wasm")
}
