package replay

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

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

	consumerID1 := "consumer1"
	consumerID2 := "consumer2"
	consumerID3 := "consumer3"

	// 1. Set up mock IBC clients for each consumer before registering consumers
	ctx := driver.App.BaseApp.NewContext(false)
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID1, &ibctmtypes.ClientState{})
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID2, &ibctmtypes.ClientState{})
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID3, &ibctmtypes.ClientState{})
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

	// Set up IBC client states in the replayer before replaying blocks
	replayerCtx := replayer.App.BaseApp.NewContext(false)
	replayer.App.IBCKeeper.ClientKeeper.SetClientState(replayerCtx, consumerID1, &ibctmtypes.ClientState{})
	replayer.App.IBCKeeper.ClientKeeper.SetClientState(replayerCtx, consumerID2, &ibctmtypes.ClientState{})
	replayer.App.IBCKeeper.ClientKeeper.SetClientState(replayerCtx, consumerID3, &ibctmtypes.ClientState{})

	// Replay all the blocks from driver and check appHash
	replayer.ReplayBlocks(t, driver.GetFinalizedBlocks())
	// After replay we should have the same apphash and last block height
	require.Equal(t, driver.GetLastState().LastBlockHeight, replayer.LastState.LastBlockHeight)
	require.Equal(t, driver.GetLastState().AppHash, replayer.LastState.AppHash)
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
