package replay

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
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

// packVerifiedDelegations packs activation of verified delegations into a single block
// with proper gas limits for each message
// It obeys all gas limits of the Babylon Genesis:
// - Every tx is less than 10M gas
// - Block will have less than 300M gas
func (d *BabylonAppDriver) packVerifiedDelegations() []*abci.ExecTxResult {
	block := d.IncludeVerifiedStakingTxInBTC(0)
	acitvationMsgs := blockWithProofsToActivationMessages(block, d.GetDriverAccountAddress())

	for i, msg := range acitvationMsgs {
		var gaslimit uint64

		if i < 5 {
			gaslimit = 1_100_000
		} else if i < 10 {
			gaslimit = 2_000_000
		} else if i < 15 {
			gaslimit = 2_700_000
		} else if i < 20 {
			gaslimit = 3_500_000
		} else if i < 25 {
			gaslimit = 4_400_000
		} else if i < 30 {
			gaslimit = 5_100_000
		} else if i < 35 {
			gaslimit = 6_000_000
		} else if i < 40 {
			gaslimit = 7_000_000
		} else if i < 45 {
			gaslimit = 7_500_000
		} else if i < 50 {
			gaslimit = 8_500_000
		} else {
			gaslimit = 10_000_000
		}

		d.SendTxWithMessagesSuccess(d.t, d.SenderInfo, gaslimit, defaultFeeCoin, msg)
		d.IncSeq()
	}

	return d.GenerateNewBlockReturnResults()
}

func (driver *BabylonAppDriver) InitCosmosConsumer(ctx sdk.Context, consumerID string) {
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID, &ibctmtypes.ClientState{})

	driver.App.IBCKeeper.ConnectionKeeper.SetConnection(
		ctx, consumerID, connectiontypes.ConnectionEnd{
			ClientId: consumerID,
		},
	)

	driver.App.IBCKeeper.ChannelKeeper.SetChannel(
		ctx, "zoneconcierge", consumerID, channeltypes.Channel{
			State:          channeltypes.OPEN,
			ConnectionHops: []string{consumerID},
		},
	)

}

func TestTooBigMulistakingPacket(t *testing.T) {
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
	driver.InitCosmosConsumer(ctx, consumerID1)
	driver.InitCosmosConsumer(ctx, consumerID2)
	driver.InitCosmosConsumer(ctx, consumerID3)
	driver.GenerateNewBlock()

	covSender := driver.CreateCovenantSender()

	// 2. Register consumers
	consumer1 := driver.RegisterConsumer(r, consumerID1)
	consumer2 := driver.RegisterConsumer(r, consumerID2)
	consumer3 := driver.RegisterConsumer(r, consumerID3)
	// Create a Babylon FP (registered without consumer ID)
	babylonFp := driver.CreateNFinalityProviderAccounts(1)[0]
	babylonFp.RegisterFinalityProvider("")

	babylonFp1 := driver.CreateNFinalityProviderAccounts(1)[0]
	babylonFp1.RegisterFinalityProvider("")
	driver.GenerateNewBlockAssertExecutionSuccess()

	// 3. Create finality providers for each consumer
	fp1s := []*FinalityProvider{
		// Create 2 FPs for consumer1
		driver.CreateFinalityProviderForConsumer(consumer1),
		driver.CreateFinalityProviderForConsumer(consumer1),
	}
	require.NotEmpty(t, fp1s)
	fp2 := driver.CreateFinalityProviderForConsumer(consumer2)
	fp3 := driver.CreateFinalityProviderForConsumer(consumer3)
	require.NotNil(t, fp3)
	// Generate blocks to process registrations
	driver.GenerateNewBlockAssertExecutionSuccess()
	staker := driver.CreateNStakerAccounts(1)[0]

	// we are sending and verifing delegations in batches of 5 to ensure
	// that covenant transaction will not go over block gas limits
	driver.SendAndVerifyNDelegations(t, staker, covSender, []*bbn.BIP340PubKey{babylonFp1.BTCPublicKey(), fp2.BTCPublicKey()}, 5)
	driver.SendAndVerifyNDelegations(t, staker, covSender, []*bbn.BIP340PubKey{babylonFp1.BTCPublicKey(), fp2.BTCPublicKey()}, 5)
	driver.SendAndVerifyNDelegations(t, staker, covSender, []*bbn.BIP340PubKey{babylonFp1.BTCPublicKey(), fp2.BTCPublicKey()}, 5)
	driver.SendAndVerifyNDelegations(t, staker, covSender, []*bbn.BIP340PubKey{babylonFp1.BTCPublicKey(), fp2.BTCPublicKey()}, 5)
	driver.SendAndVerifyNDelegations(t, staker, covSender, []*bbn.BIP340PubKey{babylonFp1.BTCPublicKey(), fp2.BTCPublicKey()}, 5)
	driver.SendAndVerifyNDelegations(t, staker, covSender, []*bbn.BIP340PubKey{babylonFp1.BTCPublicKey(), fp2.BTCPublicKey()}, 5)
	driver.SendAndVerifyNDelegations(t, staker, covSender, []*bbn.BIP340PubKey{babylonFp1.BTCPublicKey(), fp2.BTCPublicKey()}, 5)
	driver.SendAndVerifyNDelegations(t, staker, covSender, []*bbn.BIP340PubKey{babylonFp1.BTCPublicKey(), fp2.BTCPublicKey()}, 5)
	driver.SendAndVerifyNDelegations(t, staker, covSender, []*bbn.BIP340PubKey{babylonFp1.BTCPublicKey(), fp2.BTCPublicKey()}, 5)
	driver.SendAndVerifyNDelegations(t, staker, covSender, []*bbn.BIP340PubKey{babylonFp1.BTCPublicKey(), fp2.BTCPublicKey()}, 5)
	driver.SendAndVerifyNDelegations(t, staker, covSender, []*bbn.BIP340PubKey{babylonFp1.BTCPublicKey(), fp2.BTCPublicKey()}, 4)
	results := driver.packVerifiedDelegations()
	require.NotEmpty(t, results)

	// All results except the last one should be successful
	for _, result := range results[:len(results)-1] {
		require.Equal(t, uint32(0), result.Code)
	}

	lastResult := results[len(results)-1]

	// Last result should be a failure
	require.Equal(t, uint32(1), lastResult.Code)
	require.Contains(t, lastResult.Log, "IBC packet size is too large")
}

func (driver *BabylonAppDriver) SendAndVerifyNDelegations(
	t *testing.T,
	staker *Staker,
	covSender *CovenantSender,
	keys []*bbn.BIP340PubKey,
	n int,
) {

	for i := 0; i < n; i++ {
		staker.CreatePreApprovalDelegation(
			keys,
			1000,
			100000000,
		)
	}

	driver.GenerateNewBlockAssertExecutionSuccess()
	covSender.SendCovenantSignatures()
	driver.GenerateNewBlockAssertExecutionSuccess()
}
