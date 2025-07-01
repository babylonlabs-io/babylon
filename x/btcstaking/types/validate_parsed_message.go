package types

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"

	"github.com/babylonlabs-io/babylon/v3/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
)

type ParamsValidationResult struct {
	StakingOutputIdx   uint32
	UnbondingOutputIdx uint32
}

// ValidateParsedMessageAgainstTheParams validates parsed message against parameters
func ValidateParsedMessageAgainstTheParams(
	pm *ParsedCreateDelegationMessage,
	parameters *Params,
	net *chaincfg.Params,
) (*ParamsValidationResult, error) {
	// 1. Validate unbonding time first as it will be used in other checks
	// Check unbonding time (staking time from unbonding tx) is not less than min unbonding time
	if uint32(pm.UnbondingTime) != parameters.UnbondingTimeBlocks {
		return nil, ErrInvalidUnbondingTx.Wrapf("unbonding time %d must be equal to %d",
			pm.UnbondingTime, parameters.UnbondingTimeBlocks)
	}

	stakingTxHash := pm.StakingTx.Transaction.TxHash()
	covenantPks := parameters.MustGetCovenantPks()

	// 2. Validate all data related to staking tx:
	// - it has valid staking output
	// - that staking time and value are correct
	// - slashing tx is relevant to staking tx
	// - slashing tx signature is valid
	stakingInfo, err := btcstaking.BuildStakingInfo(
		pm.StakerPK.PublicKey,
		pm.FinalityProviderKeys.PublicKeys,
		covenantPks,
		parameters.CovenantQuorum,
		pm.StakingTime,
		pm.StakingValue,
		net,
	)
	if err != nil {
		return nil, ErrInvalidStakingTx.Wrapf("failed to build staking info: %v", err)
	}

	stakingOutputIdx, err := bbn.GetOutputIdxInBTCTx(pm.StakingTx.Transaction, stakingInfo.StakingOutput)

	if err != nil {
		return nil, ErrInvalidStakingTx.Wrap("staking tx does not contain expected staking output")
	}

	if uint32(pm.StakingTime) < parameters.MinStakingTimeBlocks ||
		uint32(pm.StakingTime) > parameters.MaxStakingTimeBlocks {
		return nil, ErrInvalidStakingTx.Wrapf(
			"staking time %d is out of bounds. Min: %d, Max: %d",
			pm.StakingTime,
			parameters.MinStakingTimeBlocks,
			parameters.MaxStakingTimeBlocks,
		)
	}

	if pm.StakingTx.Transaction.TxOut[stakingOutputIdx].Value < parameters.MinStakingValueSat ||
		pm.StakingTx.Transaction.TxOut[stakingOutputIdx].Value > parameters.MaxStakingValueSat {
		return nil, ErrInvalidStakingTx.Wrapf(
			"staking value %d is out of bounds. Min: %d, Max: %d",
			pm.StakingTx.Transaction.TxOut[stakingOutputIdx].Value,
			parameters.MinStakingValueSat,
			parameters.MaxStakingValueSat,
		)
	}

	if err := btcstaking.CheckSlashingTxMatchFundingTx(
		pm.StakingSlashingTx.Transaction,
		pm.StakingTx.Transaction,
		stakingOutputIdx,
		parameters.MinSlashingTxFeeSat,
		parameters.SlashingRate,
		parameters.SlashingPkScript,
		pm.StakerPK.PublicKey,
		pm.UnbondingTime,
		net,
	); err != nil {
		return nil, ErrInvalidStakingTx.Wrap(err.Error())
	}

	slashingSpendInfo, err := stakingInfo.SlashingPathSpendInfo()
	if err != nil {
		panic(fmt.Errorf("failed to construct slashing path from the staking tx: %w", err))
	}

	if err := btcstaking.VerifyTransactionSigWithOutput(
		pm.StakingSlashingTx.Transaction,
		pm.StakingTx.Transaction.TxOut[stakingOutputIdx],
		slashingSpendInfo.RevealedLeaf.Script,
		pm.StakerPK.PublicKey,
		pm.StakerStakingSlashingTxSig.BIP340Signature.MustMarshal(),
	); err != nil {
		return nil, ErrInvalidSlashingTx.Wrapf("invalid staking slashing transaction signature: %v", err)
	}

	// 3. Validate all data related to unbonding tx:
	// - it is valid BTC pre-signed transaction
	// - it has valid unbonding output
	// - slashing tx is relevant to unbonding tx
	// - slashing tx signature is valid
	if err := btcstaking.CheckPreSignedUnbondingTxSanity(
		pm.UnbondingTx.Transaction,
	); err != nil {
		return nil, ErrInvalidUnbondingTx.Wrapf("unbonding tx is not a valid pre-signed transaction: %v", err)
	}

	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		pm.StakerPK.PublicKey,
		pm.FinalityProviderKeys.PublicKeys,
		covenantPks,
		parameters.CovenantQuorum,
		pm.UnbondingTime,
		pm.UnbondingValue,
		net,
	)
	if err != nil {
		return nil, ErrInvalidUnbondingTx.Wrapf("failed to build the unbonding info: %v", err)
	}

	unbondingTx := pm.UnbondingTx.Transaction
	if !bytes.Equal(unbondingTx.TxOut[0].PkScript, unbondingInfo.UnbondingOutput.PkScript) {
		return nil, ErrInvalidUnbondingTx.
			Wrapf("the unbonding output script is not expected, expected: %x, got: %s",
				unbondingInfo.UnbondingOutput.PkScript, unbondingTx.TxOut[0].PkScript)
	}
	if unbondingTx.TxOut[0].Value != unbondingInfo.UnbondingOutput.Value {
		return nil, ErrInvalidUnbondingTx.
			Wrapf("the unbonding output value is not expected, expected: %d, got: %d",
				unbondingInfo.UnbondingOutput.Value, unbondingTx.TxOut[0].Value)
	}

	err = btcstaking.CheckSlashingTxMatchFundingTx(
		pm.UnbondingSlashingTx.Transaction,
		pm.UnbondingTx.Transaction,
		0, // unbonding output always has only 1 output
		parameters.MinSlashingTxFeeSat,
		parameters.SlashingRate,
		parameters.SlashingPkScript,
		pm.StakerPK.PublicKey,
		pm.UnbondingTime,
		net,
	)
	if err != nil {
		return nil, ErrInvalidUnbondingTx.Wrapf("err: %v", err)
	}

	unbondingSlashingSpendInfo, err := unbondingInfo.SlashingPathSpendInfo()
	if err != nil {
		panic(fmt.Errorf("failed to construct slashing path from the unbonding tx: %w", err))
	}

	if err := btcstaking.VerifyTransactionSigWithOutput(
		pm.UnbondingSlashingTx.Transaction,
		pm.UnbondingTx.Transaction.TxOut[0], // unbonding output always has only 1 output
		unbondingSlashingSpendInfo.RevealedLeaf.Script,
		pm.StakerPK.PublicKey,
		pm.StakerUnbondingSlashingSig.BIP340Signature.MustMarshal(),
	); err != nil {
		return nil, ErrInvalidSlashingTx.Wrapf("invalid unbonding slashing transaction signature: %v", err)
	}

	// 4. Check that unbonding tx input is pointing to staking tx
	if !pm.UnbondingTx.Transaction.TxIn[0].PreviousOutPoint.Hash.IsEqual(&stakingTxHash) {
		return nil, ErrInvalidUnbondingTx.Wrapf("unbonding transaction must spend staking output")
	}

	if pm.UnbondingTx.Transaction.TxIn[0].PreviousOutPoint.Index != stakingOutputIdx {
		return nil, ErrInvalidUnbondingTx.Wrapf("unbonding transaction input must spend staking output")
	}
	// 5. Check unbonding tx fees against staking tx.
	// - fee is larger than 0
	// - ubonding output value is at least `MinUnbondingValue` percent of staking output value
	if pm.UnbondingTx.Transaction.TxOut[0].Value >= pm.StakingTx.Transaction.TxOut[stakingOutputIdx].Value {
		// Note: we do not enforce any minimum fee for unbonding tx, we only require that it is larger than 0
		// Given that unbonding tx must not be replaceable, and we do not allow sending it second time, it places
		// burden on staker to choose right fee.
		// Unbonding tx should not be replaceable at babylon level (and by extension on btc level), as this would
		// allow staker to spam the network with unbonding txs, which would force covenant and finality provider to send signatures.
		return nil, ErrInvalidUnbondingTx.Wrapf("unbonding tx fee must be larger that 0")
	}

	// 6. Check that unbonding tx fee is as expected.
	unbondingTxFee := pm.StakingTx.Transaction.TxOut[stakingOutputIdx].Value - pm.UnbondingTx.Transaction.TxOut[0].Value

	if unbondingTxFee != parameters.UnbondingFeeSat {
		return nil, ErrInvalidUnbondingTx.Wrapf("unbonding tx fee must be %d, but got %d", parameters.UnbondingFeeSat, unbondingTxFee)
	}

	return &ParamsValidationResult{
		StakingOutputIdx:   stakingOutputIdx,
		UnbondingOutputIdx: 0, // unbonding output always has only 1 output
	}, nil
}
