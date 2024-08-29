package types

import (
	"fmt"

	"github.com/babylonlabs-io/babylon/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcckpttypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
)

type ParamsValidationResult struct {
	StakingOutputIdx   uint32
	UnbondingOutputIdx uint32
}

// caluculateMinimumUnbondingValue calculates minimum unbonding value basend on current staking output value
// and params.MinUnbondingRate
func caluculateMinimumUnbondingValue(
	stakingOutput *wire.TxOut,
	params *Params,
) btcutil.Amount {
	// this conversions must always succeed, as it is part of our params
	minUnbondingRate := params.MinUnbondingRate.MustFloat64()
	// Caluclate min unbonding output value based on staking output, use btc native multiplication
	minUnbondingOutputValue := btcutil.Amount(stakingOutput.Value).MulF64(minUnbondingRate)
	return minUnbondingOutputValue
}

// ValidateParams validates parsed message against parameters
func ValidateParams(
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
	slashingAddr := parameters.MustGetSlashingAddress(net)

	// 2. Validate all data related to staking tx:
	// - it has valid staking output
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

	if err := btcstaking.CheckTransactions(
		pm.StakingSlashingTx.Transaction,
		pm.StakingTx.Transaction,
		stakingOutputIdx,
		parameters.MinSlashingTxFeeSat,
		parameters.SlashingRate,
		slashingAddr,
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
		slashingAddr,
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

	minUnbondingValue := caluculateMinimumUnbondingValue(pm.StakingTx.Transaction.TxOut[stakingOutputIdx], parameters)
	if btcutil.Amount(pm.UnbondingTx.Transaction.TxOut[0].Value) < minUnbondingValue {
		return nil, ErrInvalidUnbondingTx.Wrapf("unbonding output value must be at least %s, based on staking output", minUnbondingValue)
	}

	return &ParamsValidationResult{
		StakingOutputIdx:   stakingOutputIdx,
		UnbondingOutputIdx: unbondingOutputIdx,
	}, nil
}
