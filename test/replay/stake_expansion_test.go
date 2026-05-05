package replay

import (
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testutilevents "github.com/babylonlabs-io/babylon/v4/testutil/events"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
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
					if testutilevents.IsEventType(event, &bstypes.EventCovenantSignatureReceived{}) {
						require.True(t, attributeValueNonEmpty(event, "covenant_stake_expansion_signature_hex"))
					}
				}
			}

			verifiedDels := s.Driver.GetVerifiedBTCDelegations(t)
			require.Len(t, verifiedDels, 1)
			require.NotNil(t, verifiedDels[0].StkExp)

			blockWithProofs, _ := s.Driver.IncludeVerifiedStakingTxInBTC(1)
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
						if testutilevents.IsEventType(event, &bstypes.EventCovenantSignatureReceived{}) {
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
						if testutilevents.IsEventType(event, &bstypes.EventCovenantSignatureReceived{}) {
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

				blockWithProofs, _ := s.Driver.IncludeTxsInBTCAndConfirm([]*wire.MsgTx{slashingTxMsg})
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
				blockWithProofs, _ = s.Driver.IncludeVerifiedStakingTxInBTC(1)
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

// TestStakeExpansionChildUnbondFirstNoLongerDeadlocks regression-tests the
// fix for the stake-expansion child unbond deadlock.
//
// Before the fix:
//
//	(1) Attacker submits MsgBTCUndelegate(child B, child's unbonding tx)
//	    while B is VERIFIED. The handler writes DelegatorUnbondingInfo on
//	    B (the "poison").
//	(2) Subsequent MsgBTCUndelegate(parent A, child's staking tx) fails
//	    permanently with "already unbonded" because the stake-expansion
//	    branch routes through AddBTCDelegationInclusionProof, which rejects
//	    on the poisoned child.
//	(3) Parent A stays ACTIVE on Babylon while its UTXO is gone on BTC →
//	    phantom voting power.
//
// After the fix, step (2) succeeds: the child activation is best-effort and
// failures (including "already unbonded") do not abort the parent unbonding.
// The parent's UTXO is provably spent on BTC, so its voting power is revoked
// regardless of the child's state.
func TestStakeExpansionChildUnbondFirstNoLongerDeadlocks(t *testing.T) {
	t.Parallel()
	s := setupTest(t)

	parentStkTx, parentStkTxBz, err := bbn.NewBTCTxFromHex(s.ActiveDelegations[0].StakingTxHex)
	require.NoError(t, err)
	parentStkHash := parentStkTx.TxHash().String()

	// ---- create VERIFIED stake-expansion child ----
	stakingTime := uint32(1000)
	stakingValue := int64(100_000_000)
	fundingTx := datagen.GenRandomTxWithOutputValue(s.r, 100_000)

	expandMsg := s.Staker.CreateBtcExpandMessage(
		[]*bbn.BIP340PubKey{s.Fp.BTCPublicKey()},
		stakingTime, stakingValue,
		parentStkHash,
		fundingTx, 0,
	)
	s.Staker.SendMessage(expandMsg)
	s.Driver.GenerateNewBlockAssertExecutionSuccess()
	s.CovSender.SendCovenantSignatures()
	s.Driver.GenerateNewBlockAssertExecutionSuccess()

	verifiedDels := s.Driver.GetVerifiedBTCDelegations(t)
	require.Len(t, verifiedDels, 1)
	childResp := verifiedDels[0]
	childStkTx, childStkTxBz, err := bbn.NewBTCTxFromHex(childResp.StakingTxHex)
	require.NoError(t, err)
	childUnbondingTx, _, err := bbn.NewBTCTxFromHex(childResp.UndelegationResponse.UnbondingTxHex)
	require.NoError(t, err)
	params := s.Driver.GetBTCStakingParams(t)

	// ---- poison: MsgBTCUndelegate(child, child's unbonding tx) ----
	childUnbondingTxBz, childUnbondingWitnessed := datagen.AddWitnessToUnbondingTx(
		t,
		childStkTx.TxOut[childResp.StakingOutputIdx],
		s.Staker.BTCPrivateKey,
		covenantSKs,
		params.CovenantQuorum,
		[]*btcec.PublicKey{s.Fp.BTCPrivateKey.PubKey()},
		uint16(childResp.StakingTime),
		int64(childResp.TotalSat),
		childUnbondingTx,
		s.Driver.App.BTCLightClientKeeper.GetBTCNet(),
	)
	unbondingBlock, _ := s.Driver.IncludeTxsInBTCAndConfirm([]*wire.MsgTx{childUnbondingWitnessed})
	require.Len(t, unbondingBlock.Proofs, 2)
	poisonMsg := &bstypes.MsgBTCUndelegate{
		Signer:                        s.Staker.AddressString(),
		StakingTxHash:                 childStkTx.TxHash().String(),
		StakeSpendingTx:               childUnbondingTxBz,
		StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(unbondingBlock.Proofs[1]),
		FundingTransactions:           [][]byte{childStkTxBz},
	}
	s.Staker.SendMessage(poisonMsg)
	s.Driver.GenerateNewBlockAssertExecutionSuccess()

	// child is now UNBONDED via IsUnbondedEarly
	childAfterPoison := s.Driver.GetBTCDelegation(t, childStkTx.TxHash().String())
	require.NotNil(t, childAfterPoison.UndelegationResponse.DelegatorUnbondingInfoResponse,
		"child must carry DelegatorUnbondingInfo after the poison write")
	require.Equal(t, bstypes.BTCDelegationStatus_UNBONDED.String(), childAfterPoison.StatusDesc)
	parentBefore := s.Driver.GetBTCDelegation(t, parentStkHash)
	require.Equal(t, bstypes.BTCDelegationStatus_ACTIVE.String(), parentBefore.StatusDesc,
		"parent A must still be ACTIVE before the parent unbond attempt")

	// ---- legitimate flow now SUCCEEDS post-fix ----
	expansionStakingBlock := s.Driver.IncludeTxsInBTC([]*wire.MsgTx{childStkTx})
	require.Len(t, expansionStakingBlock.Proofs, 2)
	btccParams := s.Driver.GetBTCCkptParams(t)
	for i := uint32(0); i < btccParams.BtcConfirmationDepth; i++ {
		s.Driver.ExtendBTCLcWithNEmptyBlocks(s.r, t, 1)
	}
	expansionWitnessedBz, _ := datagen.AddWitnessToStakeExpTx(
		t,
		parentStkTx.TxOut[0],
		fundingTx.TxOut[0],
		s.Staker.BTCPrivateKey,
		covenantSKs,
		params.CovenantQuorum,
		[]*btcec.PublicKey{s.Fp.BTCPrivateKey.PubKey()},
		uint16(stakingTime),
		stakingValue,
		childStkTx,
		s.Driver.App.BTCLightClientKeeper.GetBTCNet(),
	)
	parentUnbondMsg := &bstypes.MsgBTCUndelegate{
		Signer:                        s.Staker.AddressString(),
		StakingTxHash:                 parentStkHash,
		StakeSpendingTx:               expansionWitnessedBz,
		StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(expansionStakingBlock.Proofs[1]),
		FundingTransactions:           [][]byte{parentStkTxBz, expandMsg.FundingTx},
	}
	s.Staker.SendMessage(parentUnbondMsg)
	// CRITICAL: with the fix, this SUCCEEDS even though the child is poisoned.
	s.Driver.GenerateNewBlockAssertExecutionSuccess()

	// ---- final state: no more phantom voting power ----
	parentAfter := s.Driver.GetBTCDelegation(t, parentStkHash)
	require.NotNil(t, parentAfter.UndelegationResponse.DelegatorUnbondingInfoResponse,
		"parent A must now carry DelegatorUnbondingInfo (was unbonded by the legitimate flow)")
	require.Equal(t, bstypes.BTCDelegationStatus_UNBONDED.String(), parentAfter.StatusDesc,
		"FIX CONFIRMED: parent A is no longer phantom-ACTIVE")

	// child stays UNBONDED (poisoned, can never be activated, but no phantom
	// voting power either)
	childAfter := s.Driver.GetBTCDelegation(t, childStkTx.TxHash().String())
	require.Equal(t, bstypes.BTCDelegationStatus_UNBONDED.String(), childAfter.StatusDesc,
		"child stays UNBONDED — best-effort activation skipped because of the poison")

	// no ACTIVE delegations remain to be slashed against either parent or child
	activeDels := s.Driver.GetActiveBTCDelegations(t)
	for _, d := range activeDels {
		stkTx, _, derr := bbn.NewBTCTxFromHex(d.StakingTxHex)
		require.NoError(t, derr)
		require.NotEqual(t, parentStkHash, stkTx.TxHash().String(),
			"FIX CONFIRMED: parent A is no longer in the ACTIVE set")
		require.NotEqual(t, childStkTx.TxHash().String(), stkTx.TxHash().String(),
			"child is not ACTIVE either")
	}

	// ---- retry the parent unbond — must fail with status-gate error ----
	// Parent is now UNBONDED, so any subsequent MsgBTCUndelegate for it
	// hits the status gate at msg_server.go:560-562 and is rejected with
	// "cannot unbond an unbonded BTC delegation". This is the correct
	// behavior post-fix: the parent's transition is final, no double-unbond.
	s.Staker.SendMessage(parentUnbondMsg)
	retryRes := s.Driver.GenerateNewBlockAssertExecutionFailure()
	require.Len(t, retryRes, 1)
	require.Contains(t, retryRes[0].Log, "cannot unbond an unbonded BTC delegation",
		"retry of MsgBTCUndelegate(parent) is rejected by the status gate after the fix's successful unbond")
}

// TestStakeExpansionLegitimateActivationStillAtomic verifies that the
// `shouldActivateStkExp` change in BTCUndelegate does NOT regress the
// legitimate stake-expansion flow: when child has no DelegatorUnbondingInfo
// (no poison), MsgBTCUndelegate(parent, child's staking tx) must atomically
// activate the child AND unbond the parent, exactly as before.
//
// This pins the `shouldActivateStkExp = true` branch.
func TestStakeExpansionLegitimateActivationStillAtomic(t *testing.T) {
	t.Parallel()
	s := setupTest(t)

	parentStkTx, parentStkTxBz, err := bbn.NewBTCTxFromHex(s.ActiveDelegations[0].StakingTxHex)
	require.NoError(t, err)
	parentStkHash := parentStkTx.TxHash().String()

	stakingTime := uint32(1000)
	stakingValue := int64(100_000_000)
	fundingTx := datagen.GenRandomTxWithOutputValue(s.r, 100_000)

	expandMsg := s.Staker.CreateBtcExpandMessage(
		[]*bbn.BIP340PubKey{s.Fp.BTCPublicKey()},
		stakingTime, stakingValue,
		parentStkHash, fundingTx, 0,
	)
	s.Staker.SendMessage(expandMsg)
	s.Driver.GenerateNewBlockAssertExecutionSuccess()
	s.CovSender.SendCovenantSignatures()
	s.Driver.GenerateNewBlockAssertExecutionSuccess()

	verifiedDels := s.Driver.GetVerifiedBTCDelegations(t)
	require.Len(t, verifiedDels, 1)
	childResp := verifiedDels[0]
	childStkTx, _, err := bbn.NewBTCTxFromHex(childResp.StakingTxHex)
	require.NoError(t, err)
	childStkHash := childStkTx.TxHash().String()

	// pre-condition for the legitimate branch:
	//   shouldActivateStkExp = true  (child has no DelegatorUnbondingInfo)
	require.Nil(t, childResp.UndelegationResponse.DelegatorUnbondingInfoResponse,
		"child must have no DelegatorUnbondingInfo so shouldActivateStkExp will fire true")

	// confirm the child's staking tx k-deep on the BTC LC
	expansionStakingBlock, _ := s.Driver.IncludeVerifiedStakingTxInBTC(1)
	require.Len(t, expansionStakingBlock.Proofs, 2)

	params := s.Driver.GetBTCStakingParams(t)
	expansionWitnessedBz, _ := datagen.AddWitnessToStakeExpTx(
		t,
		parentStkTx.TxOut[0],
		fundingTx.TxOut[0],
		s.Staker.BTCPrivateKey,
		covenantSKs,
		params.CovenantQuorum,
		[]*btcec.PublicKey{s.Fp.BTCPrivateKey.PubKey()},
		uint16(stakingTime), stakingValue,
		childStkTx,
		s.Driver.App.BTCLightClientKeeper.GetBTCNet(),
	)

	parentUnbondMsg := &bstypes.MsgBTCUndelegate{
		Signer:                        s.Staker.AddressString(),
		StakingTxHash:                 parentStkHash,
		StakeSpendingTx:               expansionWitnessedBz,
		StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(expansionStakingBlock.Proofs[1]),
		FundingTransactions:           [][]byte{parentStkTxBz, expandMsg.FundingTx},
	}
	s.Staker.SendMessage(parentUnbondMsg)
	s.Driver.GenerateNewBlockAssertExecutionSuccess()

	// child must have been activated atomically with parent unbonding
	childAfter := s.Driver.GetBTCDelegation(t, childStkHash)
	require.Equal(t, bstypes.BTCDelegationStatus_ACTIVE.String(), childAfter.StatusDesc,
		"shouldActivateStkExp=true path: child must have flipped VERIFIED → ACTIVE atomically")
	require.Positive(t, childAfter.StartHeight, "child must have a real StartHeight from activation")
	require.Positive(t, childAfter.EndHeight, "child must have a real EndHeight from activation")

	parentAfter := s.Driver.GetBTCDelegation(t, parentStkHash)
	require.Equal(t, bstypes.BTCDelegationStatus_UNBONDED.String(), parentAfter.StatusDesc,
		"parent must be UNBONDED after legitimate stake-expansion activation")
	require.NotNil(t, parentAfter.UndelegationResponse.DelegatorUnbondingInfoResponse,
		"parent must carry DelegatorUnbondingInfo set by the stake-expansion case")
}

// TestStakeExpansionPoisonedChildFallsThroughToDefaultBranch pins the
// `shouldActivateStkExp = false` branch behavior: when the child is in
// IsUnbondedEarly state (poison), MsgBTCUndelegate(parent, child's staking
// tx) must:
//   (a) NOT call AddBTCDelegationInclusionProof (would fail and previously
//       aborted the whole handler — the pre-fix bug);
//   (b) fall through the else branch's 1-deep merkle proof check;
//   (c) trigger the `default` switch case, since spending tx is neither the
//       parent's registered unbonding tx NOR a stake-expansion-to-be-activated;
//   (d) emit EmitUnexpectedUnbondingTxEvent;
//   (e) write DelegatorUnbondingInfo on the parent with the spending tx;
//   (f) leave the child UNBONDED (poisoned) without re-touching its state.
func TestStakeExpansionPoisonedChildFallsThroughToDefaultBranch(t *testing.T) {
	t.Parallel()
	s := setupTest(t)

	parentStkTx, parentStkTxBz, err := bbn.NewBTCTxFromHex(s.ActiveDelegations[0].StakingTxHex)
	require.NoError(t, err)
	parentStkHash := parentStkTx.TxHash().String()

	stakingTime := uint32(1000)
	stakingValue := int64(100_000_000)
	fundingTx := datagen.GenRandomTxWithOutputValue(s.r, 100_000)
	expandMsg := s.Staker.CreateBtcExpandMessage(
		[]*bbn.BIP340PubKey{s.Fp.BTCPublicKey()},
		stakingTime, stakingValue, parentStkHash, fundingTx, 0,
	)
	s.Staker.SendMessage(expandMsg)
	s.Driver.GenerateNewBlockAssertExecutionSuccess()
	s.CovSender.SendCovenantSignatures()
	s.Driver.GenerateNewBlockAssertExecutionSuccess()

	verifiedDels := s.Driver.GetVerifiedBTCDelegations(t)
	require.Len(t, verifiedDels, 1)
	childResp := verifiedDels[0]
	childStkTx, childStkTxBz, err := bbn.NewBTCTxFromHex(childResp.StakingTxHex)
	require.NoError(t, err)
	childUnbondingTx, _, err := bbn.NewBTCTxFromHex(childResp.UndelegationResponse.UnbondingTxHex)
	require.NoError(t, err)
	childStkHash := childStkTx.TxHash().String()
	params := s.Driver.GetBTCStakingParams(t)

	// poison: MsgBTCUndelegate(child, child's unbonding tx). Sets
	// DelegatorUnbondingInfo on the child, status flips to UNBONDED via
	// IsUnbondedEarly. After this, `shouldActivateStkExp` will return false
	// because child.IsUnbondedEarly() == true.
	childUnbondingTxBz, childUnbondingWitnessed := datagen.AddWitnessToUnbondingTx(
		t,
		childStkTx.TxOut[childResp.StakingOutputIdx],
		s.Staker.BTCPrivateKey,
		covenantSKs,
		params.CovenantQuorum,
		[]*btcec.PublicKey{s.Fp.BTCPrivateKey.PubKey()},
		uint16(childResp.StakingTime),
		int64(childResp.TotalSat),
		childUnbondingTx,
		s.Driver.App.BTCLightClientKeeper.GetBTCNet(),
	)
	unbondingBlock, _ := s.Driver.IncludeTxsInBTCAndConfirm([]*wire.MsgTx{childUnbondingWitnessed})
	require.Len(t, unbondingBlock.Proofs, 2)
	poisonMsg := &bstypes.MsgBTCUndelegate{
		Signer:                        s.Staker.AddressString(),
		StakingTxHash:                 childStkHash,
		StakeSpendingTx:               childUnbondingTxBz,
		StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(unbondingBlock.Proofs[1]),
		FundingTransactions:           [][]byte{childStkTxBz},
	}
	s.Staker.SendMessage(poisonMsg)
	s.Driver.GenerateNewBlockAssertExecutionSuccess()

	childAfterPoison := s.Driver.GetBTCDelegation(t, childStkHash)
	require.NotNil(t, childAfterPoison.UndelegationResponse.DelegatorUnbondingInfoResponse,
		"sanity: child must carry the poison")
	require.Equal(t, bstypes.BTCDelegationStatus_UNBONDED.String(), childAfterPoison.StatusDesc,
		"sanity: child must be UNBONDED via IsUnbondedEarly")

	// Now the parent unbond. Since child.IsUnbondedEarly() is now true,
	// shouldActivateStkExp evaluates to false. The else branch's 1-deep
	// merkle check runs (no k-depth required for evidence-of-spend
	// unbonding). The default switch case fires because the spending tx is
	// neither the parent's registered unbonding tx nor a child to be
	// activated.
	expansionStakingBlock := s.Driver.IncludeTxsInBTC([]*wire.MsgTx{childStkTx})
	require.Len(t, expansionStakingBlock.Proofs, 2)
	// NB: we deliberately do NOT extend the BTC LC k-deep here. The else
	// branch only requires 1-deep merkle, so a single header with the tx
	// is sufficient. This pins that the post-fix path uses the looser
	// merkle check for the poisoned-child scenario.
	expansionWitnessedBz, _ := datagen.AddWitnessToStakeExpTx(
		t,
		parentStkTx.TxOut[0],
		fundingTx.TxOut[0],
		s.Staker.BTCPrivateKey,
		covenantSKs,
		params.CovenantQuorum,
		[]*btcec.PublicKey{s.Fp.BTCPrivateKey.PubKey()},
		uint16(stakingTime), stakingValue,
		childStkTx,
		s.Driver.App.BTCLightClientKeeper.GetBTCNet(),
	)
	parentUnbondMsg := &bstypes.MsgBTCUndelegate{
		Signer:                        s.Staker.AddressString(),
		StakingTxHash:                 parentStkHash,
		StakeSpendingTx:               expansionWitnessedBz,
		StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(expansionStakingBlock.Proofs[1]),
		FundingTransactions:           [][]byte{parentStkTxBz, expandMsg.FundingTx},
	}
	s.Staker.SendMessage(parentUnbondMsg)
	s.Driver.GenerateNewBlockAssertExecutionSuccess()

	// (e): parent has DelegatorUnbondingInfo set with the spending tx as
	// SpendStakeTx (default switch case writes the full spending tx, unlike
	// the registered-unbonding-tx case which writes empty bytes).
	parentAfter := s.Driver.GetBTCDelegation(t, parentStkHash)
	require.Equal(t, bstypes.BTCDelegationStatus_UNBONDED.String(), parentAfter.StatusDesc)
	require.NotNil(t, parentAfter.UndelegationResponse.DelegatorUnbondingInfoResponse,
		"parent must carry DelegatorUnbondingInfo from the default switch case")
	require.NotEmpty(t, parentAfter.UndelegationResponse.DelegatorUnbondingInfoResponse.SpendStakeTxHex,
		"default switch case writes the full spending tx, NOT empty bytes — distinguishes it from the "+
			"registered-unbonding-tx case where SpendStakeTx is empty")

	// (f): child state is unchanged from the poison
	childAfter := s.Driver.GetBTCDelegation(t, childStkHash)
	require.Equal(t, bstypes.BTCDelegationStatus_UNBONDED.String(), childAfter.StatusDesc,
		"child stays UNBONDED — no re-activation attempt because shouldActivateStkExp was false")
	require.Zero(t, childAfter.StartHeight, "child still has no inclusion proof")
	require.Zero(t, childAfter.EndHeight, "child still has no inclusion proof")
	require.Equal(t,
		childAfterPoison.UndelegationResponse.DelegatorUnbondingInfoResponse.SpendStakeTxHex,
		childAfter.UndelegationResponse.DelegatorUnbondingInfoResponse.SpendStakeTxHex,
		"child's DelegatorUnbondingInfo must not change during parent unbonding")
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
	fp1.RegisterFinalityProvider()
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
