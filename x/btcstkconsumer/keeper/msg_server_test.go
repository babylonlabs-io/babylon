package keeper_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	wasmtest "github.com/babylonlabs-io/babylon/v4/wasmbinding/test"
	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
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
			MaxMultiStakedFps:   consumerRegister.MaxMultiStakedFps,
		})
		require.NoError(t, err)
		// check that the consumer is registered
		consumerRegister2, err := bscKeeper.GetConsumerRegister(ctx, consumerRegister.ConsumerId)
		require.NoError(t, err)
		require.Equal(t, consumerRegister.String(), consumerRegister2.String())

		/*
			Test registering consumer with invalid max_multi_staked_fps (zero)
		*/
		// generate a random consumer register
		consumerRegister = datagen.GenRandomCosmosConsumerRegister(r)
		// mock IBC light client
		babylonApp.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerRegister.ConsumerId, &ibctmtypes.ClientState{})
		// Register the consumer with zero max_multi_staked_fps
		_, err = msgServer.RegisterConsumer(ctx, &types.MsgRegisterConsumer{
			ConsumerId:          consumerRegister.ConsumerId,
			ConsumerName:        consumerRegister.ConsumerName,
			ConsumerDescription: consumerRegister.ConsumerDescription,
			MaxMultiStakedFps:   0,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, types.ErrInvalidMaxMultiStakedFps)

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
			MaxMultiStakedFps:            consumerRegister.MaxMultiStakedFps,
			EthL2FinalityContractAddress: contractAddr.String(),
		})
		require.NoError(t, err)
		// check that the consumer is registered
		consumerRegister2, err = bscKeeper.GetConsumerRegister(ctx, consumerRegister.ConsumerId)
		require.NoError(t, err)
		require.Equal(t, consumerRegister.String(), consumerRegister2.String())
		require.Equal(t, types.ConsumerType_ETH_L2, consumerRegister2.Type())
		require.Equal(t, consumerRegister.MaxMultiStakedFps, consumerRegister2.MaxMultiStakedFps)
	})
}

func mockSmartContract(t *testing.T, ctx sdk.Context, babylonApp *app.BabylonApp) sdk.AccAddress {
	return wasmtest.DeployTestContract(t, ctx, babylonApp, sdk.AccAddress{}, "../../../wasmbinding/testdata/artifacts/testdata.wasm")
}
