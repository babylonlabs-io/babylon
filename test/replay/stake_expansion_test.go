package replay

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"
)

func TestExpandBTCDelegation(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	driver.GenerateNewBlock()

	covSender := driver.CreateCovenantSender()
	require.NotNil(t, covSender)

	infos := driver.CreateNFinalityProviderAccounts(1)
	fp1 := infos[0]

	sinfos := driver.CreateNStakerAccounts(1)
	s1 := sinfos[0]

	fp1.RegisterFinalityProvider()
	driver.GenerateNewBlockAssertExecutionSuccess()

	var (
		stakingTime  = uint32(1000)
		stakingValue = int64(100000000)
	)
	s1.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		stakingTime,
		stakingValue,
	)
	driver.GenerateNewBlockAssertExecutionSuccess()

	covSender.SendCovenantSignatures()
	driver.GenerateNewBlockAssertExecutionSuccess()

	driver.ActivateVerifiedDelegations(1)
	activeDelegations := driver.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 1)

	prevStkTx, prevStkTxBz, err := bbn.NewBTCTxFromHex(activeDelegations[0].StakingTxHex)
	require.NoError(t, err)

	btcExpMsg := s1.CreateBtcStakeExpand(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		stakingTime,
		stakingValue,
		prevStkTx,
	)
	driver.GenerateNewBlockAssertExecutionSuccess()

	// A BTC delegation with stake expansion should be creatd
	// with pending state
	pendingDelegations := driver.GetPendingBTCDelegations(t)
	require.Len(t, pendingDelegations, 1)

	require.NotNil(t, pendingDelegations[0].StkExp)

	covSender.SendCovenantSignatures()
	results := driver.GenerateNewBlockAssertExecutionSuccessWithResults()
	require.NotEmpty(t, results)

	for _, result := range results {
		for _, event := range result.Events {
			if event.Type == "babylon.btcstaking.v1.EventCovenantSignatureReceived" {
				require.True(t, attributeValueNonEmpty(event, "covenant_stake_expansion_signature_hex"))
			}
		}
	}

	// After getting covenant sigs, stake expansion delegation
	// should be verified
	verifiedDels := driver.GetVerifiedBTCDelegations(t)
	require.Len(t, verifiedDels, 1)
	require.NotNil(t, verifiedDels[0].StkExp)

	// wait stake expansion Tx be 'k' deep in BTC
	blockWithProofs, _ := driver.IncludeVerifiedStakingTxInBTC(1)
	require.Len(t, blockWithProofs.Proofs, 2)

	// Send MsgBTCUndelegate for the first delegation
	// to activate stake expansion
	spendingTx, err := bbn.NewBTCTxFromBytes(btcExpMsg.StakingTx)
	require.NoError(t, err)

	fundingTx, err := bbn.NewBTCTxFromBytes(btcExpMsg.FundingTx)
	require.NoError(t, err)

	params := driver.GetBTCStakingParams(t)
	spendingTxWithWitnessBz, _ := datagen.AddWitnessToStakeExpTx(
		t,
		prevStkTx.TxOut[0],
		fundingTx.TxOut[0],
		s1.BTCPrivateKey,
		covenantSKs,
		params.CovenantQuorum,
		[]*btcec.PublicKey{fp1.BTCPrivateKey.PubKey()},
		uint16(stakingTime),
		stakingValue,
		spendingTx,
		driver.App.BTCLightClientKeeper.GetBTCNet(),
	)

	msg := &bstypes.MsgBTCUndelegate{
		Signer:                        s1.AddressString(),
		StakingTxHash:                 prevStkTx.TxHash().String(),
		StakeSpendingTx:               spendingTxWithWitnessBz,
		StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(blockWithProofs.Proofs[1]),
		FundingTransactions:           [][]byte{prevStkTxBz, btcExpMsg.FundingTx},
	}

	s1.SendMessage(msg)
	driver.GenerateNewBlockAssertExecutionSuccess()

	// After unbonding the initial delegation
	// the stake expansion should become active
	// and the initial delegation should become unbonded
	activeDelegations = driver.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 1)
	require.NotNil(t, activeDelegations[0].StkExp)

	unbondedDelegations := driver.GetUnbondedBTCDelegations(t)
	require.Len(t, unbondedDelegations, 1)
}

// TODO: Convert this to table test testing different cases
func TestRejectStakeExpansionUsingPreviousStakingOutput(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	driver.GenerateNewBlock()

	covSender := driver.CreateCovenantSender()
	infos := driver.CreateNFinalityProviderAccounts(1)
	fp1 := infos[0]
	fp1.RegisterFinalityProvider()
	driver.GenerateNewBlockAssertExecutionSuccess()

	sinfos := driver.CreateNStakerAccounts(1)
	s1 := sinfos[0]
	require.NotNil(t, s1)

	msg := s1.CreateDelegationMessage(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		1000,
		100000000,
	)

	msg1 := s1.CreateDelegationMessage(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		1000,
		100000000,
	)

	driver.ConfirmStakingTransactionOnBTC([]*bstypes.MsgCreateBTCDelegation{msg, msg1})
	require.NotNil(t, msg.StakingTxInclusionProof)
	require.NotNil(t, msg1.StakingTxInclusionProof)
	s1.SendMessage(msg)
	s1.SendMessage(msg1)
	driver.GenerateNewBlockAssertExecutionSuccess()
	// Activate through covenant signatures
	covSender.SendCovenantSignatures()
	driver.GenerateNewBlockAssertExecutionSuccess()

	activeDelegations := driver.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 2)

	prevStkTx, _, err := bbn.NewBTCTxFromHex(activeDelegations[0].StakingTxHex)
	require.NoError(t, err)
	prevStkTxHash := prevStkTx.TxHash()

	// using other staking tx as funding tx
	fundingTx, _, err := bbn.NewBTCTxFromHex(activeDelegations[1].StakingTxHex)
	require.NoError(t, err)

	stakeExpandMsg := s1.CreateBtcExpandMessage(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		1000,
		100000000,
		prevStkTxHash.String(),
		fundingTx,
		0,
	)

	s1.SendMessage(stakeExpandMsg)
	res := driver.GenerateNewBlockAssertExecutionFailure()
	require.Len(t, res, 1)
	require.Equal(t, res[0].Log, "failed to execute message; message index: 0: rpc error: code = InvalidArgument desc = the funding output cannot be a staking output")
}
