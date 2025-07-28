package replay

import (
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func TestExpandBTCDelegation(t *testing.T) {
	testCases := []struct {
		name         string
		fundingSetup func(s *testSetup) (*wire.MsgTx, uint32)
	}{
		{
			name: "with random funding tx",
			fundingSetup: func(s *testSetup) (*wire.MsgTx, uint32) {
				return datagen.GenRandomTxWithOutputValue(s.r, 100000), 0
			},
		},
		{
			name: "using a UTXO that is not a staking output from another delegation",
			fundingSetup: func(s *testSetup) (*wire.MsgTx, uint32) {
				fundingTx, _, err := bbn.NewBTCTxFromHex(s.ActiveDelegations[1].StakingTxHex)
				require.NoError(t, err)
				require.Len(t, fundingTx.TxOut, 2)
				return fundingTx, 1
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := setupTest(t)

			prevStkTx, prevStkTxBz, err := bbn.NewBTCTxFromHex(s.ActiveDelegations[0].StakingTxHex)
			require.NoError(t, err)

			stakingTime := uint32(1000)
			stakingValue := int64(100000000)

			fundingTx, fundingTxOutIdx := tc.fundingSetup(s)

			btcExpMsg := s.Staker.CreateBtcExpandMessage(
				[]*bbn.BIP340PubKey{s.Fp.BTCPublicKey()},
				stakingTime,
				stakingValue,
				prevStkTx.TxHash().String(),
				fundingTx,
				fundingTxOutIdx,
			)
			s.Staker.SendMessage(btcExpMsg)

			s.Driver.GenerateNewBlockAssertExecutionSuccess()

			pendingDelegations := s.Driver.GetPendingBTCDelegations(t)
			require.Len(t, pendingDelegations, 1)
			require.NotNil(t, pendingDelegations[0].StkExp)

			s.CovSender.SendCovenantSignatures()
			results := s.Driver.GenerateNewBlockAssertExecutionSuccessWithResults()
			require.NotEmpty(t, results)

			for _, result := range results {
				for _, event := range result.Events {
					if event.Type == "babylon.btcstaking.v1.EventCovenantSignatureReceived" {
						require.True(t, attributeValueNonEmpty(event, "covenant_stake_expansion_signature_hex"))
					}
				}
			}

			verifiedDels := s.Driver.GetVerifiedBTCDelegations(t)
			require.Len(t, verifiedDels, 1)
			require.NotNil(t, verifiedDels[0].StkExp)

			blockWithProofs := s.Driver.IncludeVerifiedStakingTxInBTC(1)
			require.Len(t, blockWithProofs.Proofs, 2)

			spendingTx, err := bbn.NewBTCTxFromBytes(btcExpMsg.StakingTx)
			require.NoError(t, err)

			params := s.Driver.GetBTCStakingParams(t)
			spendingTxWithWitnessBz, _ := datagen.AddWitnessToStakeExpTx(
				t,
				prevStkTx.TxOut[0],
				fundingTx.TxOut[fundingTxOutIdx],
				s.Staker.BTCPrivateKey,
				covenantSKs,
				params.CovenantQuorum,
				[]*btcec.PublicKey{s.Fp.BTCPrivateKey.PubKey()},
				uint16(stakingTime),
				stakingValue,
				spendingTx,
				s.Driver.App.BTCLightClientKeeper.GetBTCNet(),
			)

			msg := &bstypes.MsgBTCUndelegate{
				Signer:                        s.Staker.AddressString(),
				StakingTxHash:                 prevStkTx.TxHash().String(),
				StakeSpendingTx:               spendingTxWithWitnessBz,
				StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(blockWithProofs.Proofs[1]),
				FundingTransactions:           [][]byte{prevStkTxBz, btcExpMsg.FundingTx},
			}

			s.Staker.SendMessage(msg)
			s.Driver.GenerateNewBlockAssertExecutionSuccess()

			resp := s.Driver.GetBTCDelegation(t, spendingTx.TxHash().String())
			require.NotNil(t, resp)
			require.NotNil(t, resp.StkExp)
			require.Equal(t, resp.StatusDesc, bstypes.BTCDelegationStatus_ACTIVE.String())

			unbondedDelegations := s.Driver.GetUnbondedBTCDelegations(t)
			require.Len(t, unbondedDelegations, 1)

			// there should be 2 active delegations now
			// the other delegation added on the test setup
			// and the one we just expanded
			activeDelegations := s.Driver.GetActiveBTCDelegations(t)
			require.Len(t, activeDelegations, 2)
		})
	}
}

func TestInvalidStakeExpansion(t *testing.T) {
	testCases := []struct {
		name     string
		testCase func(s *testSetup)
	}{
		{
			name: "using a staking output as funding output",
			testCase: func(s *testSetup) {
				prevStkTx, _, err := bbn.NewBTCTxFromHex(s.ActiveDelegations[0].StakingTxHex)
				require.NoError(t, err)
				prevStkTxHash := prevStkTx.TxHash()

				fundingTx, _, err := bbn.NewBTCTxFromHex(s.ActiveDelegations[1].StakingTxHex)
				require.NoError(t, err)

				stakeExpandMsg := s.Staker.CreateBtcExpandMessage(
					[]*bbn.BIP340PubKey{s.Fp.BTCPublicKey()},
					1000,
					100000000,
					prevStkTxHash.String(),
					fundingTx,
					s.ActiveDelegations[1].StakingOutputIdx,
				)

				s.Staker.SendMessage(stakeExpandMsg)
				res := s.Driver.GenerateNewBlockAssertExecutionFailure()
				require.Len(t, res, 1)
				require.Equal(t, res[0].Log, "failed to execute message; message index: 0: rpc error: code = InvalidArgument desc = the funding output cannot be a staking output")
			},
		},
		{
			name: "report staking output spending before it is k-deep in BTC",
			testCase: func(s *testSetup) {
				prevStkTx, prevStkTxBz, err := bbn.NewBTCTxFromHex(s.ActiveDelegations[0].StakingTxHex)
				require.NoError(t, err)
				prevStkTxHash := prevStkTx.TxHash()

				fundingTx := datagen.GenRandomTxWithOutputValue(s.r, 100000)

				// Create a stake expansion message
				stakeExpandMsg := s.Staker.CreateBtcExpandMessage(
					[]*bbn.BIP340PubKey{s.Fp.BTCPublicKey()},
					1000,
					100000000,
					prevStkTxHash.String(),
					fundingTx,
					0,
				)

				s.Staker.SendMessage(stakeExpandMsg)
				s.Driver.GenerateNewBlockAssertExecutionSuccess()

				// Submit covenant signatures
				s.CovSender.SendCovenantSignatures()
				results := s.Driver.GenerateNewBlockAssertExecutionSuccessWithResults()
				require.NotEmpty(t, results)

				for _, result := range results {
					for _, event := range result.Events {
						if event.Type == "babylon.btcstaking.v1.EventCovenantSignatureReceived" {
							require.True(t, attributeValueNonEmpty(event, "covenant_stake_expansion_signature_hex"))
						}
					}
				}

				verifiedDels := s.Driver.GetVerifiedBTCDelegations(t)
				require.Len(t, verifiedDels, 1)
				require.NotNil(t, verifiedDels[0].StkExp)

				stkExpStakingTx, err := bbn.NewBTCTxFromBytes(stakeExpandMsg.StakingTx)
				require.NoError(t, err)

				// add tx to BTC and submit headers with proofs
				blockWithProofs := s.Driver.IncludeTxsInBTC([]*wire.MsgTx{stkExpStakingTx})
				require.Len(t, blockWithProofs.Proofs, 2)

				stakingTime := uint32(1000)
				stakingValue := int64(100000000)

				params := s.Driver.GetBTCStakingParams(t)
				spendingTxWithWitnessBz, _ := datagen.AddWitnessToStakeExpTx(
					t,
					prevStkTx.TxOut[0],
					fundingTx.TxOut[0],
					s.Staker.BTCPrivateKey,
					covenantSKs,
					params.CovenantQuorum,
					[]*btcec.PublicKey{s.Fp.BTCPrivateKey.PubKey()},
					uint16(stakingTime),
					stakingValue,
					stkExpStakingTx,
					s.Driver.App.BTCLightClientKeeper.GetBTCNet(),
				)

				// Try to unbond the delegation to activate the stake expansion
				// but the stake expansion tx is not k-deep in BTC yet
				msg := &bstypes.MsgBTCUndelegate{
					Signer:                        s.Staker.AddressString(),
					StakingTxHash:                 prevStkTx.TxHash().String(),
					StakeSpendingTx:               spendingTxWithWitnessBz,
					StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(blockWithProofs.Proofs[1]),
					FundingTransactions:           [][]byte{prevStkTxBz, stakeExpandMsg.FundingTx},
				}

				s.Staker.SendMessage(msg)
				res := s.Driver.GenerateNewBlockAssertExecutionFailure()
				require.Len(t, res, 1)
				require.Contains(t, res[0].Log, "invalid inclusion proof: not k-deep")

				// check that previous delegations are still active
				// and the stake expansion is still verified
				activeDelegations := s.Driver.GetActiveBTCDelegations(t)
				require.Len(t, activeDelegations, 2)

				verifiedDels = s.Driver.GetVerifiedBTCDelegations(t)
				require.Len(t, verifiedDels, 1)
				require.NotNil(t, verifiedDels[0].StkExp)
			},
		},
		{
			name: "user unbonds before activating stake expansion",
			testCase: func(s *testSetup) {
				prevStkTx, prevStkTxBz, err := bbn.NewBTCTxFromHex(s.ActiveDelegations[0].StakingTxHex)
				require.NoError(t, err)
				prevStkTxHash := prevStkTx.TxHash()

				fundingTx := datagen.GenRandomTxWithOutputValue(s.r, 100000)
				stakingTime := uint32(1000)
				stakingValue := int64(100000000)

				// Create a stake expansion message
				stakeExpandMsg := s.Staker.CreateBtcExpandMessage(
					[]*bbn.BIP340PubKey{s.Fp.BTCPublicKey()},
					stakingTime,
					stakingValue,
					prevStkTxHash.String(),
					fundingTx,
					0,
				)

				s.Staker.SendMessage(stakeExpandMsg)
				s.Driver.GenerateNewBlockAssertExecutionSuccess()

				// Submit covenant signatures
				s.CovSender.SendCovenantSignatures()
				results := s.Driver.GenerateNewBlockAssertExecutionSuccessWithResults()
				require.NotEmpty(t, results)

				for _, result := range results {
					for _, event := range result.Events {
						if event.Type == "babylon.btcstaking.v1.EventCovenantSignatureReceived" {
							require.True(t, attributeValueNonEmpty(event, "covenant_stake_expansion_signature_hex"))
						}
					}
				}

				verifiedDels := s.Driver.GetVerifiedBTCDelegations(t)
				require.Len(t, verifiedDels, 1)
				require.NotNil(t, verifiedDels[0].StkExp)

				// Unbond the delegation before the stake expansion is activated
				params := s.Driver.GetBTCStakingParams(t)
				unbondingTx, _, err := bbn.NewBTCTxFromHex(s.ActiveDelegations[0].UndelegationResponse.UnbondingTxHex)
				require.NoError(t, err)
				unbondingTxBytes, slashingTxMsg := datagen.AddWitnessToUnbondingTx(
					t,
					prevStkTx.TxOut[0],
					s.Staker.BTCPrivateKey,
					covenantSKs,
					params.CovenantQuorum,
					[]*btcec.PublicKey{s.Fp.BTCPrivateKey.PubKey()},
					uint16(stakingTime),
					stakingValue,
					unbondingTx,
					s.Driver.App.BTCLightClientKeeper.GetBTCNet(),
				)

				blockWithProofs := s.Driver.IncludeTxsInBTCAncConfirm([]*wire.MsgTx{slashingTxMsg})
				require.Len(t, blockWithProofs.Proofs, 2)

				msg := &bstypes.MsgBTCUndelegate{
					Signer:                        s.Staker.AddressString(),
					StakingTxHash:                 prevStkTx.TxHash().String(),
					StakeSpendingTx:               unbondingTxBytes,
					StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(blockWithProofs.Proofs[1]),
					FundingTransactions:           [][]byte{prevStkTxBz},
				}
				s.Staker.SendMessage(msg)
				s.Driver.GenerateNewBlockAssertExecutionSuccessWithResults()

				// check that the delegation is unbonded
				unbondedDelegations := s.Driver.GetUnbondedBTCDelegations(t)
				require.Len(t, unbondedDelegations, 1)

				// Add tx to BTC and submit headers with proofs
				// In practice this would not be possible because the
				// staking output should be already spent by the unbonding tx
				stkExpStakingTx, err := bbn.NewBTCTxFromBytes(stakeExpandMsg.StakingTx)
				require.NoError(t, err)
				blockWithProofs = s.Driver.IncludeVerifiedStakingTxInBTC(1)
				require.Len(t, blockWithProofs.Proofs, 2)

				spendingTxWithWitnessBz, _ := datagen.AddWitnessToStakeExpTx(
					t,
					prevStkTx.TxOut[0],
					fundingTx.TxOut[0],
					s.Staker.BTCPrivateKey,
					covenantSKs,
					params.CovenantQuorum,
					[]*btcec.PublicKey{s.Fp.BTCPrivateKey.PubKey()},
					uint16(stakingTime),
					stakingValue,
					stkExpStakingTx,
					s.Driver.App.BTCLightClientKeeper.GetBTCNet(),
				)

				// Try to unbond the delegation to activate the stake expansion
				// but the original delegation is already unbonded
				msg = &bstypes.MsgBTCUndelegate{
					Signer:                        s.Staker.AddressString(),
					StakingTxHash:                 prevStkTx.TxHash().String(),
					StakeSpendingTx:               spendingTxWithWitnessBz,
					StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(blockWithProofs.Proofs[1]),
					FundingTransactions:           [][]byte{prevStkTxBz, stakeExpandMsg.FundingTx},
				}

				s.Staker.SendMessage(msg)
				res := s.Driver.GenerateNewBlockAssertExecutionFailure()
				require.Len(t, res, 1)
				require.Contains(t, res[0].Log, "cannot unbond an unbonded BTC delegation")

				// check that previous extra delegation is still active
				// and the stake expansion is still verified
				activeDelegations := s.Driver.GetActiveBTCDelegations(t)
				require.Len(t, activeDelegations, 1)

				verifiedDels = s.Driver.GetVerifiedBTCDelegations(t)
				require.Len(t, verifiedDels, 1)
				require.NotNil(t, verifiedDels[0].StkExp)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := setupTest(t)
			tc.testCase(s)
		})
	}
}

type testSetup struct {
	r                 *rand.Rand
	Driver            *BabylonAppDriver
	CovSender         *CovenantSender
	Staker            *Staker
	Fp                *FinalityProvider
	ActiveDelegations []*bstypes.BTCDelegationResponse
}

func setupTest(t *testing.T) *testSetup {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	driver.GenerateNewBlock()

	covSender := driver.CreateCovenantSender()
	infos := driver.CreateNFinalityProviderAccounts(1)
	fp1 := infos[0]
	fp1.RegisterFinalityProvider("")
	driver.GenerateNewBlockAssertExecutionSuccess()

	sinfos := driver.CreateNStakerAccounts(1)
	s1 := sinfos[0]
	require.NotNil(t, s1)

	stakingTime := uint32(1000)
	stakingValue := int64(100000000)
	msg := s1.CreateDelegationMessage(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		stakingTime,
		stakingValue,
	)

	msg1 := s1.CreateDelegationMessageWithChange(
		[]*bbn.BIP340PubKey{fp1.BTCPublicKey()},
		stakingTime,
		stakingValue*2,
		stakingValue, // change amount
	)

	driver.ConfirmStakingTransactionOnBTC([]*bstypes.MsgCreateBTCDelegation{msg, msg1})
	require.NotNil(t, msg.StakingTxInclusionProof)
	require.NotNil(t, msg1.StakingTxInclusionProof)
	s1.SendMessage(msg)
	s1.SendMessage(msg1)
	driver.GenerateNewBlockAssertExecutionSuccess()
	covSender.SendCovenantSignatures()
	driver.GenerateNewBlockAssertExecutionSuccess()

	activeDelegations := driver.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 2)

	sort.Slice(activeDelegations, func(i, j int) bool {
		return activeDelegations[i].TotalSat < activeDelegations[j].TotalSat
	})

	return &testSetup{
		r:                 r,
		Driver:            driver,
		CovSender:         covSender,
		Staker:            s1,
		Fp:                fp1,
		ActiveDelegations: activeDelegations,
	}
}
