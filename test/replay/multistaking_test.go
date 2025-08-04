package replay

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/stretchr/testify/require"
)

func TestMultiConsumerDelegation(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)

	const consumerID1 = "consumer1"
	const consumerID2 = "consumer2"
	const consumerID3 = "consumer3"

	// 1. Set up mock IBC clients for each consumer before registering consumers
	ctx := driver.App.BaseApp.NewContext(false)
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID1, &ibctmtypes.ClientState{})
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID2, &ibctmtypes.ClientState{})
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID3, &ibctmtypes.ClientState{})
	// open channel for the consumers
	OpenChannelForConsumer(ctx, driver.App, consumerID1)
	OpenChannelForConsumer(ctx, driver.App, consumerID2)
	OpenChannelForConsumer(ctx, driver.App, consumerID3)
	driver.GenerateNewBlock()

	// 2. Register consumers
	consumer1 := driver.RegisterConsumer(r, consumerID1)
	consumer2 := driver.RegisterConsumer(r, consumerID2)
	consumer3 := driver.RegisterConsumer(r, consumerID3)
	// Create a Babylon FP (registered without consumer ID)
	babylonFp := driver.CreateNFinalityProviderAccounts(1)[0]
	babylonFp.RegisterFinalityProvider("")

	// 3. Create finality providers for each consumer
	fp1s := []*FinalityProvider{
		// Create 2 FPs for consumer1
		driver.CreateFinalityProviderForConsumer(consumer1),
		driver.CreateFinalityProviderForConsumer(consumer1),
	}
	fp2 := driver.CreateFinalityProviderForConsumer(consumer2)
	fp3 := driver.CreateFinalityProviderForConsumer(consumer3)
	// Generate blocks to process registrations
	driver.GenerateNewBlockAssertExecutionSuccess()
	staker := driver.CreateNStakerAccounts(1)[0]

	// 4. Create a delegation with three consumer FPs and one Babylon FP
	staker.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1s[0].BTCPublicKey(), fp2.BTCPublicKey(), fp3.BTCPublicKey(), babylonFp.BTCPublicKey()},
		1000,
		100000000,
	)

	// 5. Create a valid delegation with 2 FPs (including Babylon FP)
	staker.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{babylonFp.BTCPublicKey(), fp1s[0].BTCPublicKey()},
		1000,
		100000000,
	)
	driver.GenerateNewBlockAssertExecutionSuccess()

	// 6. Replay all blocks and verify state
	replayer := NewBlockReplayer(t, replayerTempDir)

	// Set up IBC client states and channels in the replayer before replaying blocks
	replayerCtx := replayer.App.BaseApp.NewContext(false)
	replayer.App.IBCKeeper.ClientKeeper.SetClientState(replayerCtx, consumerID1, &ibctmtypes.ClientState{})
	replayer.App.IBCKeeper.ClientKeeper.SetClientState(replayerCtx, consumerID2, &ibctmtypes.ClientState{})
	replayer.App.IBCKeeper.ClientKeeper.SetClientState(replayerCtx, consumerID3, &ibctmtypes.ClientState{})
	// Open channels for consumers
	OpenChannelForConsumer(replayerCtx, replayer.App, consumerID1)
	OpenChannelForConsumer(replayerCtx, replayer.App, consumerID2)
	OpenChannelForConsumer(replayerCtx, replayer.App, consumerID3)

	// Replay all the blocks from driver and check appHash
	replayer.ReplayBlocks(t, driver.GetFinalizedBlocks())
	// After replay we should have the same apphash and last block height
	require.Equal(t, driver.GetLastState().LastBlockHeight, replayer.LastState.LastBlockHeight)
	require.Equal(t, driver.GetLastState().AppHash, replayer.LastState.AppHash)
}

func TestConsumerFpCreationWithoutChannel(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)

	const consumerID1 = "consumer1"

	// Set up mock IBC clients for consumer before registering consumers
	ctx := driver.App.BaseApp.NewContext(false)
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID1, &ibctmtypes.ClientState{})

	driver.GenerateNewBlock()

	// Register consumer
	consumer1 := driver.RegisterConsumer(r, consumerID1)
	// Create a Babylon FP (registered without consumer ID)
	babylonFp := driver.CreateNFinalityProviderAccounts(1)[0]
	babylonFp.RegisterFinalityProvider("")

	// should fail if we try to create an FP and there's no open channel
	driver.CreateFinalityProviderForConsumer(consumer1)
	res := driver.GenerateNewBlockAssertExecutionFailure()
	require.Len(t, res, 1)
	require.Contains(t, res[0].Log, "consumer does not have an open IBC channel")
}

func TestMultiConsumerDelegationTooManyKeys(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)

	consumerID1 := "consumer1"

	// 1. Set up mock IBC clients for each consumer before registering consumers
	ctx := driver.App.BaseApp.NewContext(false)
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID1, &ibctmtypes.ClientState{})
	driver.GenerateNewBlock()

	// 2. Register consumers
	consumer1 := driver.RegisterConsumer(r, consumerID1)
	// open channel for the consumer
	OpenChannelForConsumer(driver.Ctx(), driver.App, consumerID1)
	driver.GenerateNewBlock()

	// Create a Babylon FP (registered without consumer ID)
	babylonFp := driver.CreateNFinalityProviderAccounts(1)[0]
	babylonFp.RegisterFinalityProvider("")

	params := driver.GetBTCStakingParams(t)

	var fps []*FinalityProvider
	for i := 0; i < int(params.MaxFinalityProviders); i++ {
		fps = append(fps, driver.CreateFinalityProviderForConsumer(consumer1))
	}

	// Generate blocks to process registrations
	driver.GenerateNewBlockAssertExecutionSuccess()
	staker := driver.CreateNStakerAccounts(1)[0]

	var bbnFpKeys []*bbn.BIP340PubKey
	bbnFpKeys = append(bbnFpKeys, babylonFp.BTCPublicKey())
	for _, fp := range fps {
		bbnFpKeys = append(bbnFpKeys, fp.BTCPublicKey())
	}

	// 4. Create a delegation with three consumer FPs and one Babylon FP
	staker.CreatePreApprovalDelegation(
		bbnFpKeys,
		1000,
		100000000,
	)
	expectedLog := fmt.Sprintf("failed to execute message; message index: 0: number of finality providers %d is greater than max finality providers %d: the BTC staking tx is not valid", len(bbnFpKeys), params.MaxFinalityProviders)

	txResults := driver.GenerateNewBlockAssertExecutionFailure()
	require.Len(t, txResults, 1)
	require.Equal(t, uint32(1108), txResults[0].Code)
	require.Contains(t, txResults[0].Log, expectedLog)
}

func TestAdditionalGasCostForMultiStakedDelegation(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)

	consumerID1 := "consumer1"
	consumerID2 := "consumer2"

	// 1. Set up mock IBC clients for each consumer before registering consumers
	ctx := driver.App.BaseApp.NewContext(false)
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID1, &ibctmtypes.ClientState{})
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID2, &ibctmtypes.ClientState{})
	// open channels for the consumers
	OpenChannelForConsumer(ctx, driver.App, consumerID1)
	OpenChannelForConsumer(ctx, driver.App, consumerID2)
	driver.GenerateNewBlock()

	// 2. Register consumers
	consumer1 := driver.RegisterConsumer(r, consumerID1)
	consumer2 := driver.RegisterConsumer(r, consumerID2)

	// Create a Babylon FP (registered without consumer ID)
	babylonFps := driver.CreateNFinalityProviderAccounts(2)
	babylonFps[0].RegisterFinalityProvider("")
	babylonFps[1].RegisterFinalityProvider("")

	fp1 := driver.CreateFinalityProviderForConsumer(consumer1)
	require.NotNil(t, fp1)
	fp2 := driver.CreateFinalityProviderForConsumer(consumer2)
	require.NotNil(t, fp2)

	multiStakedFpKeys := []*bbn.BIP340PubKey{
		fp1.BTCPublicKey(),
		fp2.BTCPublicKey(),
	}

	// Generate blocks to process registrations
	driver.GenerateNewBlockAssertExecutionSuccess()
	staker := driver.CreateNStakerAccounts(1)[0]

	// 4. Create a delegation with three consumer FPs and one Babylon FP
	staker.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{
			babylonFps[0].BTCPublicKey(),
		},
		1000,
		100000000,
	)
	txResults1 := driver.GenerateNewBlockAssertExecutionSuccessWithResults()
	require.Len(t, txResults1, 1)
	require.Equal(t, uint32(0), txResults1[0].Code)

	staker.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{
			babylonFps[1].BTCPublicKey(),
			multiStakedFpKeys[0],
			multiStakedFpKeys[1],
		},
		1000,
		100000000,
	)

	txResults2 := driver.GenerateNewBlockAssertExecutionSuccessWithResults()
	require.Len(t, txResults2, 1)
	require.Equal(t, uint32(0), txResults2[0].Code)

	params := driver.GetBTCStakingParams(t)
	minimalGasDifference := bstypes.GasCostPerMultiStakedFP * len(multiStakedFpKeys) * len(params.CovenantPks)
	// We cannot use equal as multistaked delegations use more gas by default, though
	// the difference is small enough so that `minimalGasDifference` is much larger than it
	require.GreaterOrEqual(t, txResults2[0].GasUsed-txResults1[0].GasUsed, int64(minimalGasDifference))
}
