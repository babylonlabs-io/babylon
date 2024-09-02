package types

import (
	"fmt"

	"github.com/babylonlabs-io/babylon/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcckpttypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/btcsuite/btcd/chaincfg"
)

type ParamsValidationResult struct {
	StakingOutputIdx   uint32
	UnbondingOutputIdx uint32
}

// ValidateParsedMessageAgainstTheParams validates parsed message against parameters
func ValidateParsedMessageAgainstTheParams(
	pm *ParsedCreateDelegationMessage,
	parameters *Params,
	btcheckpointParamseters *btcckpttypes.Params,
	net *chaincfg.Params,
) (*ParamsValidationResult, error) {
	// 1. Validate unbonding time first as it will be used in other checks
	minUnbondingTime := MinimumUnbondingTime(parameters, btcheckpointParamseters)
	// Check unbonding time (staking time from unbonding tx) is larger than min unbonding time
	// which is larger value from:
	// - MinUnbondingTime
	// - CheckpointFinalizationTimeout
	if uint64(pm.UnbondingTime) <= minUnbondingTime {
		return nil, ErrInvalidUnbondingTx.Wrapf("unbonding time %d must be larger than %d", pm.UnbondingTime, minUnbondingTime)
	}

	stakingTxHash := pm.StakingTx.Transaction.TxHash()
	covenantPks := parameters.MustGetCovenantPks()

	// 2. Validate all data related to staking tx:
	// - it has valid staking output
	// - that staking time and value are correct
	// - slashing tx is relevent to staking tx
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
		return nil, ErrInvalidStakingTx.Wrapf("err: %v", err)
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

	if err := btcstaking.CheckTransactions(
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
		return nil, ErrInvalidSlashingTx.Wrapf("invalid delegator signature: %v", err)
	}

	// 3. Validate all data related to unbonding tx:
	// - it has valid unbonding output
	// - slashing tx is relevent to unbonding tx
	// - slashing tx signature is valid
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
		return nil, ErrInvalidUnbondingTx.Wrapf("err: %v", err)
	}

	unbondingOutputIdx, err := bbn.GetOutputIdxInBTCTx(pm.UnbondingTx.Transaction, unbondingInfo.UnbondingOutput)
	if err != nil {
		return nil, ErrInvalidUnbondingTx.Wrapf("unbonding tx does not contain expected unbonding output")
	}

	err = btcstaking.CheckTransactions(
		pm.UnbondingSlashingTx.Transaction,
		pm.UnbondingTx.Transaction,
		unbondingOutputIdx,
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
		pm.UnbondingTx.Transaction.TxOut[unbondingOutputIdx],
		unbondingSlashingSpendInfo.RevealedLeaf.Script,
		pm.StakerPK.PublicKey,
		pm.StakerUnbondingSlashingSig.BIP340Signature.MustMarshal(),
	); err != nil {
		return nil, ErrInvalidSlashingTx.Wrapf("invalid delegator signature: %v", err)
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
	// - ubonding output value is is at leat `MinUnbondingValue` percent of staking output value
	if pm.UnbondingTx.Transaction.TxOut[0].Value >= pm.StakingTx.Transaction.TxOut[stakingOutputIdx].Value {
		// Note: we do not enfore any minimum fee for unbonding tx, we only require that it is larger than 0
		// Given that unbonding tx must not be replacable and we do not allow sending it second time, it places
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
		UnbondingOutputIdx: unbondingOutputIdx,
	}, nil
}
