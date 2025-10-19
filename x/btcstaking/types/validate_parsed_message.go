package types

import (
	"bytes"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"

	"github.com/btcsuite/btcd/chaincfg"

	"github.com/babylonlabs-io/babylon/v4/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
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
	var stakingOutputIdx uint32

	// 1. Validate unbonding time first as it will be used in other checks
	// Check unbonding time (staking time from unbonding tx) is not less than min unbonding time
	if uint32(pm.UnbondingTime) != parameters.UnbondingTimeBlocks {
		return nil, ErrInvalidUnbondingTx.Wrapf("unbonding time %d must be equal to %d",
			pm.UnbondingTime, parameters.UnbondingTimeBlocks)
	}

	stakingTxHash := pm.StakingTx.Transaction.TxHash()
	covenantPks := parameters.MustGetCovenantPks()

	// handle multi-sig btc delegation and single-sig btc delegation separately
	if pm.ExtraStakerInfo != nil {
		// this is the M-of-N multisig btc delegation
		if len(pm.ExtraStakerInfo.StakerBTCPkList.PublicKeys) < 1 || pm.ExtraStakerInfo.StakerQuorum < 1 {
			return nil, ErrInvalidMultisigInfo.Wrapf("number of staker btc pk list and staker quorum must be greater than 0, got: %d, %d",
				len(pm.ExtraStakerInfo.StakerBTCPkList.PublicKeys), pm.ExtraStakerInfo.StakerQuorum,
			)
		}

		// validate staker quorum and length of staker btc pk list doesn't exceed max M-of-N
		if int(parameters.MaxStakerNum) < len(pm.ExtraStakerInfo.StakerBTCPkList.PublicKeys)+1 ||
			parameters.MaxStakerQuorum < pm.ExtraStakerInfo.StakerQuorum {
			return nil, ErrInvalidMultisigInfo.Wrapf("invalid M-of-N parameters: staker quorum %d, staker num %d, max %d-of-%d",
				pm.ExtraStakerInfo.StakerQuorum, len(pm.ExtraStakerInfo.StakerBTCPkList.PublicKeys)+1, parameters.MaxStakerQuorum, parameters.MaxStakerNum)
		}

		// construct the complete list of staker pubkeys from `ExtraStakerInfo` and `StakerPk`
		stakerKeys := pm.ExtraStakerInfo.StakerBTCPkList.PublicKeys
		stakerKeys = append(stakerKeys, pm.StakerPK.PublicKey)
		stakerQuorum := pm.ExtraStakerInfo.StakerQuorum

		// construct pubkey -> bip340 signature map for each slashing tx and unbonding slashing tx
		slashingPubkey2Sig := make(map[*btcec.PublicKey][]byte)
		slashingPubkey2Sig[pm.StakerPK.PublicKey] = pm.StakerStakingSlashingTxSig.BIP340Signature.MustMarshal()
		for _, si := range pm.ExtraStakerInfo.StakerStakingSlashingSigs {
			slashingPubkey2Sig[si.PublicKey.PublicKey] = si.Sig.BIP340Signature.MustMarshal()
		}

		unbondingSlashingPubkey2Sig := make(map[*btcec.PublicKey][]byte)
		unbondingSlashingPubkey2Sig[pm.StakerPK.PublicKey] = pm.StakerUnbondingSlashingSig.BIP340Signature.MustMarshal()
		for _, si := range pm.ExtraStakerInfo.StakerUnbondingSlashingSigs {
			unbondingSlashingPubkey2Sig[si.PublicKey.PublicKey] = si.Sig.BIP340Signature.MustMarshal()
		}

		// compare the length of pubkey -> sig map and the `StakerQuorum`
		if len(slashingPubkey2Sig) != int(stakerQuorum) || len(unbondingSlashingPubkey2Sig) != int(stakerQuorum) {
			return nil, ErrInvalidMultisigInfo.Wrapf("invalid %d-of-%d signatures: %d slashing signatures, %d unbonding slashing signatures",
				pm.ExtraStakerInfo.StakerQuorum, len(pm.ExtraStakerInfo.StakerBTCPkList.PublicKeys)+1, len(slashingPubkey2Sig), len(unbondingSlashingPubkey2Sig))
		}

		// 2. Validate all data related to staking tx:
		// - it has valid staking output
		// - that staking time and value are correct
		// - slashing tx is relevant to staking tx
		// - slashing tx signature is valid
		multisigStakingInfo, err := btcstaking.BuildMultisigStakingInfo(
			stakerKeys,
			stakerQuorum,
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

		stakingOutputIdx, err = bbn.GetOutputIdxInBTCTx(pm.StakingTx.Transaction, multisigStakingInfo.StakingOutput)

		if err != nil {
			return nil, ErrInvalidStakingTx.Wrap("staking tx does not contain expected staking output")
		}

		if err := validateStakingTimeAndValueAgainstTheParams(pm, parameters, stakingOutputIdx); err != nil {
			return nil, err
		}

		if err := btcstaking.CheckSlashingTxMatchFundingTxMultisig(
			pm.StakingSlashingTx.Transaction,
			pm.StakingTx.Transaction,
			stakingOutputIdx,
			parameters.MinSlashingTxFeeSat,
			parameters.SlashingRate,
			parameters.SlashingPkScript,
			stakerKeys,
			stakerQuorum,
			pm.UnbondingTime,
			net,
		); err != nil {
			return nil, ErrInvalidStakingTx.Wrap(err.Error())
		}

		slashingSpendInfo, err := multisigStakingInfo.SlashingPathSpendInfo()
		if err != nil {
			panic(fmt.Errorf("failed to construct slashing path from the staking tx: %w", err))
		}

		if err := btcstaking.VerifyTransactionMultiSigWithOutput(
			pm.StakingSlashingTx.Transaction,
			pm.StakingTx.Transaction.TxOut[stakingOutputIdx],
			slashingSpendInfo.RevealedLeaf.Script,
			slashingPubkey2Sig,
		); err != nil {
			return nil, ErrInvalidSlashingTx.Wrapf("invalid delegator signature: %v", err)
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

		multisigUnbondingInfo, err := btcstaking.BuildMultisigUnbondingInfo(
			stakerKeys,
			stakerQuorum,
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
		if !bytes.Equal(unbondingTx.TxOut[0].PkScript, multisigUnbondingInfo.UnbondingOutput.PkScript) {
			return nil, ErrInvalidUnbondingTx.
				Wrapf("the unbonding output script is not expected, expected: %x, got: %s",
					multisigUnbondingInfo.UnbondingOutput.PkScript, unbondingTx.TxOut[0].PkScript)
		}
		if unbondingTx.TxOut[0].Value != multisigUnbondingInfo.UnbondingOutput.Value {
			return nil, ErrInvalidUnbondingTx.
				Wrapf("the unbonding output value is not expected, expected: %d, got: %d",
					multisigUnbondingInfo.UnbondingOutput.Value, unbondingTx.TxOut[0].Value)
		}

		err = btcstaking.CheckSlashingTxMatchFundingTxMultisig(
			pm.UnbondingSlashingTx.Transaction,
			pm.UnbondingTx.Transaction,
			0, // unbonding output always has only 1 output
			parameters.MinSlashingTxFeeSat,
			parameters.SlashingRate,
			parameters.SlashingPkScript,
			stakerKeys,
			stakerQuorum,
			pm.UnbondingTime,
			net,
		)
		if err != nil {
			return nil, ErrInvalidUnbondingTx.Wrapf("err: %v", err)
		}

		unbondingSlashingSpendInfo, err := multisigUnbondingInfo.SlashingPathSpendInfo()
		if err != nil {
			panic(fmt.Errorf("failed to construct slashing path from the unbonding tx: %w", err))
		}

		if err := btcstaking.VerifyTransactionMultiSigWithOutput(
			pm.UnbondingSlashingTx.Transaction,
			pm.UnbondingTx.Transaction.TxOut[0], // unbonding output always has only 1 output
			unbondingSlashingSpendInfo.RevealedLeaf.Script,
			unbondingSlashingPubkey2Sig,
		); err != nil {
			return nil, ErrInvalidSlashingTx.Wrapf("invalid delegator signature: %v", err)
		}
	} else {
		// this is the original single-sig btc delegation
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

		stakingOutputIdx, err = bbn.GetOutputIdxInBTCTx(pm.StakingTx.Transaction, stakingInfo.StakingOutput)

		if err != nil {
			return nil, ErrInvalidStakingTx.Wrap("staking tx does not contain expected staking output")
		}

		if err := validateStakingTimeAndValueAgainstTheParams(pm, parameters, stakingOutputIdx); err != nil {
			return nil, err
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
			return nil, ErrInvalidSlashingTx.Wrapf("invalid delegator signature: %v", err)
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
			return nil, ErrInvalidSlashingTx.Wrapf("invalid delegator signature: %v", err)
		}
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

func validateStakingTimeAndValueAgainstTheParams(
	pm *ParsedCreateDelegationMessage,
	parameters *Params,
	stakingOutputIdx uint32,
) error {
	if uint32(pm.StakingTime) < parameters.MinStakingTimeBlocks ||
		uint32(pm.StakingTime) > parameters.MaxStakingTimeBlocks {
		return ErrInvalidStakingTx.Wrapf(
			"staking time %d is out of bounds. Min: %d, Max: %d",
			pm.StakingTime,
			parameters.MinStakingTimeBlocks,
			parameters.MaxStakingTimeBlocks,
		)
	}

	if pm.StakingTx.Transaction.TxOut[stakingOutputIdx].Value < parameters.MinStakingValueSat ||
		pm.StakingTx.Transaction.TxOut[stakingOutputIdx].Value > parameters.MaxStakingValueSat {
		return ErrInvalidStakingTx.Wrapf(
			"staking value %d is out of bounds. Min: %d, Max: %d",
			pm.StakingTx.Transaction.TxOut[stakingOutputIdx].Value,
			parameters.MinStakingValueSat,
			parameters.MaxStakingValueSat,
		)
	}

	return nil
}
