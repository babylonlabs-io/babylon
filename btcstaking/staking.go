package btcstaking

import (
	"bytes"
	"encoding/hex"
	"fmt"

	sdkmath "cosmossdk.io/math"
	asig "github.com/babylonlabs-io/babylon/v3/crypto/schnorr-adaptor-signature"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/mempool"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

const (
	// MaxTxVersion is the maximum transaction version allowed in Babylon system.
	// Changing that constant will require upgrade in the future, if we ever need
	// to support v3 transactions.
	MaxTxVersion = 2

	MaxStandardTxWeight = 400000
)

// buildSlashingTxFromOutpoint builds a valid slashing transaction by creating a new Bitcoin transaction that slashes a portion
// of staked funds and directs them to a specified slashing address. The transaction also includes a change output sent back to
// the specified change address. The slashing rate determines the proportion of staked funds to be slashed.
//
// Parameters:
//   - stakingOutput: The staking output to be spent in the transaction.
//   - stakingAmount: The amount of staked funds in the staking output.
//   - fee: The transaction fee to be paid.
//   - slashingAddress: The Bitcoin address to which the slashed funds will be sent.
//   - changeAddress: The Bitcoin address to receive the change from the transaction.
//   - slashingRate: The rate at which the staked funds will be slashed, expressed as a decimal.
//
// Returns:
//   - *wire.MsgTx: The constructed slashing transaction without a script signature or witness.
//   - error: An error if any validation or construction step fails.
func buildSlashingTxFromOutpoint(
	stakingOutput wire.OutPoint,
	stakingAmount, fee int64,
	slashingPkScript []byte,
	changeAddress btcutil.Address,
	slashingRate sdkmath.LegacyDec,
) (*wire.MsgTx, error) {
	// Validate staking amount
	if stakingAmount <= 0 {
		return nil, fmt.Errorf("staking amount must be larger than 0")
	}

	// Validate slashing rate
	if !IsSlashingRateValid(slashingRate) {
		return nil, ErrInvalidSlashingRate
	}

	if len(slashingPkScript) == 0 {
		return nil, fmt.Errorf("slashing pk script must not be empty")
	}

	// Calculate the amount to be slashed
	slashingRateFloat64, err := slashingRate.Float64()
	if err != nil {
		return nil, fmt.Errorf("error converting slashing rate to float64: %w", err)
	}
	slashingAmount := btcutil.Amount(stakingAmount).MulF64(slashingRateFloat64)
	if slashingAmount <= 0 {
		return nil, ErrInsufficientSlashingAmount
	}

	// Calculate the change amount
	changeAmount := btcutil.Amount(stakingAmount) - slashingAmount - btcutil.Amount(fee)
	if changeAmount <= 0 {
		return nil, ErrInsufficientChangeAmount
	}
	// Generate script for change address
	changeAddrScript, err := txscript.PayToAddrScript(changeAddress)
	if err != nil {
		return nil, err
	}

	// Create a new btc transaction
	tx := wire.NewMsgTx(wire.TxVersion)
	// TODO: this builds input with sequence number equal to MaxTxInSequenceNum, which
	// means this tx is not replaceable.
	input := wire.NewTxIn(&stakingOutput, nil, nil)
	tx.AddTxIn(input)
	tx.AddTxOut(wire.NewTxOut(int64(slashingAmount), slashingPkScript))
	tx.AddTxOut(wire.NewTxOut(int64(changeAmount), changeAddrScript))

	// Verify that the none of the outputs is a dust output.
	for _, out := range tx.TxOut {
		if isOPReturn(out.PkScript) {
			continue
		}

		if mempool.IsDust(out, mempool.DefaultMinRelayTxFee) {
			return nil, ErrDustOutputFound
		}
	}

	return tx, nil
}

func getPossibleStakingOutput(
	stakingTx *wire.MsgTx,
	stakingOutputIdx uint32,
) (*wire.TxOut, error) {
	if stakingTx == nil {
		return nil, fmt.Errorf("provided staking transaction must not be nil")
	}

	if int(stakingOutputIdx) >= len(stakingTx.TxOut) {
		return nil, fmt.Errorf("invalid staking output index %d, tx has %d outputs", stakingOutputIdx, len(stakingTx.TxOut))
	}

	stakingOutput := stakingTx.TxOut[stakingOutputIdx]

	if !txscript.IsPayToTaproot(stakingOutput.PkScript) {
		return nil, fmt.Errorf("must be pay to taproot output")
	}

	return stakingOutput, nil
}

// BuildSlashingTxFromStakingTxStrict constructs a valid slashing transaction using information from a staking transaction,
// a specified staking output index, and additional parameters such as slashing and change addresses, transaction fee,
// staking script, script version, and network. This function performs stricter validation compared to BuildSlashingTxFromStakingTx.
//
// Parameters:
//   - stakingTx: The staking transaction from which the staking output is to be used for slashing.
//   - stakingOutputIdx: The index of the staking output in the staking transaction.
//   - stakerPk: public key of the staker i.e the btc holder who can spend staking output after lock time
//   - slashChangeLockTime: lock time for change output in slashing transaction
//   - fee: The transaction fee to be paid.
//   - slashingRate: The rate at which the staked funds will be slashed, expressed as a decimal.
//   - net: The network on which transactions should take place (e.g., mainnet, testnet).
//
// Returns:
//   - *wire.MsgTx: The constructed slashing transaction without script signature or witness.
//   - error: An error if any validation or construction step fails.
func BuildSlashingTxFromStakingTxStrict(
	stakingTx *wire.MsgTx,
	stakingOutputIdx uint32,
	slashingPkScript []byte,
	stakerPk *btcec.PublicKey,
	slashChangeLockTime uint16,
	fee int64,
	slashingRate sdkmath.LegacyDec,
	net *chaincfg.Params,
) (*wire.MsgTx, error) {
	// Get the staking output at the specified index from the staking transaction
	stakingOutput, err := getPossibleStakingOutput(stakingTx, stakingOutputIdx)
	if err != nil {
		return nil, err
	}

	// Create an OutPoint for the staking output
	stakingTxHash := stakingTx.TxHash()
	stakingOutpoint := wire.NewOutPoint(&stakingTxHash, stakingOutputIdx)

	// Create taproot address committing to timelock script
	si, err := BuildRelativeTimelockTaprootScript(
		stakerPk,
		slashChangeLockTime,
		net,
	)

	if err != nil {
		return nil, err
	}

	// Build slashing tx with the staking output information
	return buildSlashingTxFromOutpoint(
		*stakingOutpoint,
		stakingOutput.Value, fee,
		slashingPkScript, si.TapAddress,
		slashingRate)
}

// IsTransferTx Transfer transaction is a transaction which:
// - has exactly one input
// - has exactly one output
func IsTransferTx(tx *wire.MsgTx) error {
	if tx == nil {
		return fmt.Errorf("transfer transaction must have cannot be nil")
	}

	if len(tx.TxIn) != 1 {
		return fmt.Errorf("transfer transaction must have exactly one input")
	}

	if len(tx.TxOut) != 1 {
		return fmt.Errorf("transfer transaction must have exactly one output")
	}

	return nil
}

// IsSimpleTransfer Simple transfer transaction is a transaction which:
// - has exactly one input
// - has exactly one output
// - is not replaceable
// - does not have any locktime
func IsSimpleTransfer(tx *wire.MsgTx) error {
	if err := IsTransferTx(tx); err != nil {
		return fmt.Errorf("invalid simple transfer tx: %w", err)
	}

	if tx.TxIn[0].Sequence != wire.MaxTxInSequenceNum {
		return fmt.Errorf("simple transfer tx must not be replaceable")
	}

	if tx.LockTime != 0 {
		return fmt.Errorf("simple transfer tx must not have locktime")
	}
	return nil
}

// CheckPreSignedTxSanity performs basic checks on a pre-signed transaction:
// - the transaction is not nil.
// - the transaction obeys basic BTC rules.
// - the transaction has exactly numInputs inputs.
// - the transaction has exactly numOutputs outputs.
// - the transaction lock time is 0.
// - the transaction version is between minTxVersion and maxTxVersion.
// - each input has a sequence number equal to MaxTxInSequenceNum.
// - each input has an empty signature script.
// - each input has an empty witness.
func CheckPreSignedTxSanity(
	tx *wire.MsgTx,
	numInputs, numOutputs uint32,
	minTxVersion, maxTxVersion int32,
) error {
	if tx == nil {
		return fmt.Errorf("tx must not be nil")
	}

	transaction := btcutil.NewTx(tx)

	if err := blockchain.CheckTransactionSanity(transaction); err != nil {
		return fmt.Errorf("btc transaction do not obey BTC rules: %w", err)
	}

	if len(tx.TxIn) != int(numInputs) {
		return fmt.Errorf("tx must have exactly %d inputs", numInputs)
	}

	if len(tx.TxOut) != int(numOutputs) {
		return fmt.Errorf("tx must have exactly %d outputs", numOutputs)
	}

	// this requirement makes every pre-signed tx final
	if tx.LockTime != 0 {
		return fmt.Errorf("pre-signed tx must not have locktime")
	}

	if tx.Version > maxTxVersion || tx.Version < minTxVersion {
		return fmt.Errorf("tx version must be between %d and %d", minTxVersion, maxTxVersion)
	}

	txWeight := blockchain.GetTransactionWeight(transaction)

	// Check that the transaction weight does not exceed the maximum standard tx weight
	// alternative would be to require len(in.Witness) == 0 for all inptus.
	if txWeight > MaxStandardTxWeight {
		return fmt.Errorf("tx weight must not exceed %d", MaxStandardTxWeight)
	}

	for _, in := range tx.TxIn {
		if in.Sequence != wire.MaxTxInSequenceNum {
			return fmt.Errorf("pre-signed tx must not be replaceable")
		}

		// We require this to be 0, as all babylon pre-signed transactions use
		// witness
		if len(in.SignatureScript) != 0 {
			return fmt.Errorf("pre-signed tx must not have signature script")
		}
	}

	return nil
}

func CheckPreSignedUnbondingTxSanity(tx *wire.MsgTx) error {
	return CheckPreSignedTxSanity(
		tx,
		1,
		1,
		// Unbonding tx is always version 2
		MaxTxVersion,
		MaxTxVersion,
	)
}

func CheckPreSignedSlashingTxSanity(tx *wire.MsgTx) error {
	return CheckPreSignedTxSanity(
		tx,
		1,
		2,
		// slashing tx version can be between 1 and 2
		1,
		MaxTxVersion,
	)
}

func isOPReturn(script []byte) bool {
	return len(script) > 0 && script[0] == txscript.OP_RETURN
}

// validateSlashingTx performs basic checks on a slashing transaction:
// - the slashing transaction is not nil.
// - the slashing transaction has exactly one input.
// - the slashing transaction is non-replaceable.
// - the lock time of the slashing transaction is 0.
// - the slashing transaction has exactly two outputs, and:
//   - the first output must pay to the provided slashing address.
//   - the first output must pay at least (staking output value * slashing rate) to the slashing address.
//   - neither of the outputs are considered dust.
//
// - the min fee for slashing tx is preserved
func validateSlashingTx(
	slashingTx *wire.MsgTx,
	slashingPkScript []byte,
	slashingRate sdkmath.LegacyDec,
	slashingTxMinFee, stakingOutputValue int64,
	stakerPk *btcec.PublicKey,
	slashingChangeLockTime uint16,
	net *chaincfg.Params,
) error {
	if err := CheckPreSignedSlashingTxSanity(slashingTx); err != nil {
		return fmt.Errorf("invalid slashing tx: %w", err)
	}

	// Verify that at least staking output value * slashing rate is slashed.
	slashingRateFloat64, err := slashingRate.Float64()
	if err != nil {
		return fmt.Errorf("error converting slashing rate to float64: %w", err)
	}
	minSlashingAmount := btcutil.Amount(stakingOutputValue).MulF64(slashingRateFloat64)
	if btcutil.Amount(slashingTx.TxOut[0].Value) < minSlashingAmount {
		return fmt.Errorf("slashing transaction must slash at least staking output value * slashing rate")
	}

	if !bytes.Equal(slashingTx.TxOut[0].PkScript, slashingPkScript) {
		return fmt.Errorf("slashing transaction must pay to the provided slashing address")
	}

	// Verify that the second output pays to the taproot address which locks funds for
	// slashingChangeLockTime
	si, err := BuildRelativeTimelockTaprootScript(
		stakerPk,
		slashingChangeLockTime,
		net,
	)

	if err != nil {
		return fmt.Errorf("error creating change timelock script: %w", err)
	}

	if !bytes.Equal(slashingTx.TxOut[1].PkScript, si.PkScript) {
		return fmt.Errorf("invalid slashing tx change output pkscript, expected: %s, got: %s", hex.EncodeToString(si.PkScript), hex.EncodeToString(slashingTx.TxOut[1].PkScript))
	}

	// Verify that the none of the outputs is a dust output.
	for _, out := range slashingTx.TxOut {
		// OP_RETURN outputs can be dust and considered standard
		if isOPReturn(out.PkScript) {
			continue
		}

		if mempool.IsDust(out, mempool.DefaultMinRelayTxFee) {
			return ErrDustOutputFound
		}
	}

	/*
		Check Fees
	*/
	// Check that values of slashing and staking transaction are larger than 0
	if slashingTx.TxOut[0].Value <= 0 || stakingOutputValue <= 0 {
		return fmt.Errorf("values of slashing and staking transaction must be larger than 0")
	}

	// Calculate the sum of output values in the slashing transaction.
	slashingTxOutSum := int64(0)
	for _, out := range slashingTx.TxOut {
		slashingTxOutSum += out.Value
	}

	// Ensure that the staking transaction value is larger than the sum of slashing transaction output values.
	if stakingOutputValue <= slashingTxOutSum {
		return fmt.Errorf("slashing transaction must not spend more than staking transaction")
	}

	// Ensure that the slashing transaction fee is larger than the specified minimum fee.
	if stakingOutputValue-slashingTxOutSum < slashingTxMinFee {
		return fmt.Errorf("slashing transaction fee must be larger than %d", slashingTxMinFee)
	}

	return nil
}

// CheckSlashingTxMatchFundingTx validates all relevant data of slashing and funding transaction.
// - both transactions are valid from pov of BTC rules
// - slashing transaction is valid
// - slashing transaction input hash is pointing to funding transaction hash
// - slashing transaction input index is pointing to funding transaction output committing to the script
func CheckSlashingTxMatchFundingTx(
	slashingTx *wire.MsgTx,
	fundingTransaction *wire.MsgTx,
	fundingOutputIdx uint32,
	slashingTxMinFee int64,
	slashingRate sdkmath.LegacyDec,
	slashingPkScript []byte,
	stakerPk *btcec.PublicKey,
	slashingChangeLockTime uint16,
	net *chaincfg.Params,
) error {
	if slashingTx == nil || fundingTransaction == nil {
		return fmt.Errorf("slashing and funding transactions must not be nil")
	}

	if err := blockchain.CheckTransactionSanity(btcutil.NewTx(fundingTransaction)); err != nil {
		return fmt.Errorf("funding transaction does not obey BTC rules: %w", err)
	}

	// Check if slashing tx min fee is valid
	if slashingTxMinFee <= 0 {
		return fmt.Errorf("slashing transaction min fee must be larger than 0")
	}

	// Check if slashing rate is in the valid range (0,1)
	if !IsSlashingRateValid(slashingRate) {
		return ErrInvalidSlashingRate
	}

	if int(fundingOutputIdx) >= len(fundingTransaction.TxOut) {
		return fmt.Errorf("invalid funding output index %d, tx has %d outputs", fundingOutputIdx, len(fundingTransaction.TxOut))
	}

	stakingOutput := fundingTransaction.TxOut[fundingOutputIdx]
	// 3. Check if slashing transaction is valid
	if err := validateSlashingTx(
		slashingTx,
		slashingPkScript,
		slashingRate,
		slashingTxMinFee,
		stakingOutput.Value,
		stakerPk,
		slashingChangeLockTime,
		net); err != nil {
		return err
	}

	// 4. Check that slashing transaction input is pointing to staking transaction
	stakingTxHash := fundingTransaction.TxHash()
	if !slashingTx.TxIn[0].PreviousOutPoint.Hash.IsEqual(&stakingTxHash) {
		return fmt.Errorf("slashing transaction must spend staking output")
	}

	// 5. Check that index of the fund output matches index of the input in slashing transaction
	if slashingTx.TxIn[0].PreviousOutPoint.Index != fundingOutputIdx {
		return fmt.Errorf("slashing transaction input must spend staking output")
	}
	return nil
}

// SignTxWithOneScriptSpendInputFromTapLeaf signs transaction with one input coming
// from script spend output.
// It does not do any validations, expect that txToSign has exactly one input.
func SignTxWithOneScriptSpendInputFromTapLeaf(
	txToSign *wire.MsgTx,
	fundingOutput *wire.TxOut,
	privKey *btcec.PrivateKey,
	tapLeaf txscript.TapLeaf,
) (*schnorr.Signature, error) {
	if err := validateSignTxScriptSpendInput(txToSign, fundingOutput, privKey); err != nil {
		return nil, err
	}

	if len(txToSign.TxIn) != 1 {
		return nil, fmt.Errorf("tx to sign must have exactly one input")
	}

	return signTaprootScriptSpendInput(
		txToSign,
		0,
		map[wire.OutPoint]*wire.TxOut{
			txToSign.TxIn[0].PreviousOutPoint: fundingOutput,
		},
		privKey,
		tapLeaf,
		txscript.SigHashDefault,
	)
}

func validateSignTxScriptSpendInput(
	txToSign *wire.MsgTx,
	fundingOutput *wire.TxOut,
	privKey *btcec.PrivateKey,
) error {
	if txToSign == nil {
		return fmt.Errorf("tx to sign must not be nil")
	}

	if fundingOutput == nil {
		return fmt.Errorf("funding output must not be nil")
	}

	if privKey == nil {
		return fmt.Errorf("private key must not be nil")
	}

	return nil
}

// SignTxWithOneScriptSpendInputFromScript signs transaction with one input coming
// from script spend output with provided script.
// It does not do any validations, expect that txToSign has exactly one input.
func SignTxWithOneScriptSpendInputFromScript(
	txToSign *wire.MsgTx,
	fundingOutput *wire.TxOut,
	privKey *btcec.PrivateKey,
	script []byte,
) (*schnorr.Signature, error) {
	tapLeaf := txscript.NewBaseTapLeaf(script)
	return SignTxWithOneScriptSpendInputFromTapLeaf(txToSign, fundingOutput, privKey, tapLeaf)
}

// GetSignatureForFirstScriptSpendWithTwoInputsFromScript signs transaction with two input coming
// from script spend output with provided script.
// It does not do any validations, expect that txToSign has exactly two inputs.
// In the context of a stake expansion the idx 0 `fundingOutputToSignIdx0`
// funding output would be the previous active staking transaction that
// needs the covenant signatures to spend the BTC and the other funding
// output `fundingOutputIdx1` would be an TxOut responsible to pay for fees
// and optionally increasing the amount staked to that delegation.
func GetSignatureForFirstScriptSpendWithTwoInputsFromScript(
	txToSign *wire.MsgTx,
	fundingOutputToSignIdx0 *wire.TxOut,
	fundingOutputIdx1 *wire.TxOut,
	privKey *btcec.PrivateKey,
	script []byte,
) (*schnorr.Signature, error) {
	tapLeaf := txscript.NewBaseTapLeaf(script)
	return GetSignatureForFirstScriptSpendWithTwoInputsFromTapLeaf(txToSign, fundingOutputToSignIdx0, fundingOutputIdx1, privKey, tapLeaf)
}

// GetSignatureForFirstScriptSpendWithTwoInputsFromTapLeaf signs transaction with two inputs coming
// from script spend output.
// It does not do any validations, expect that txToSign has exactly two inputs.
func GetSignatureForFirstScriptSpendWithTwoInputsFromTapLeaf(
	txToSign *wire.MsgTx,
	fundingOutputToSignIdx0 *wire.TxOut,
	fundingOutputIdx1 *wire.TxOut,
	privKey *btcec.PrivateKey,
	tapLeaf txscript.TapLeaf,
) (*schnorr.Signature, error) {
	if err := validateSignTxScriptSpendInput(txToSign, fundingOutputToSignIdx0, privKey); err != nil {
		return nil, err
	}

	if fundingOutputIdx1 == nil {
		return nil, fmt.Errorf("funding output of idx 1 must not be nil")
	}

	if len(txToSign.TxIn) != 2 {
		return nil, fmt.Errorf("tx to sign must have exactly two inputs")
	}

	// returns the schnorr signature of the signature over the idx zero of the inputs
	// of the message to sign. In the context of a stake expansion the idx 0
	// funding output would be the previous active staking transaction that
	// needs the covenant signatures to spend the BTC and the other funding
	// output `fundingOutputIdx1` would be an TxOut responsible to pay for fees
	// and optionally increasing the amount staked to that delegation.
	return signTaprootScriptSpendInput(
		txToSign,
		0,
		map[wire.OutPoint]*wire.TxOut{
			txToSign.TxIn[0].PreviousOutPoint: fundingOutputToSignIdx0,
			txToSign.TxIn[1].PreviousOutPoint: fundingOutputIdx1,
		},
		privKey,
		tapLeaf,
		txscript.SigHashDefault,
	)
}

// signTaprootScriptSpendInput generates a Schnorr signature for a specific input of a transaction,
// using Taproot script path spending (via a provided TapLeaf).
//
// It supports signing arbitrary inputs, as long as their corresponding previous outputs are supplied
// via the `prevOutputs` map. The signature is generated using BIP-340 rules and is suitable for spending
// Taproot outputs via a control block and leaf script.
//
// Parameters:
//   - txToSign: The transaction to sign.
//   - inputIdxToSign: Index of the input to sign (0-based).
//   - prevOutputs: A map of previous outputs indexed by their OutPoints. This must include the prevout
//     corresponding to the input being signed.
//   - privKey: The private key used to generate the Schnorr signature.
//   - tapLeaf: The TapLeaf representing the script path being used to authorize the spend.
//   - sigHashType: The signature hash type (e.g., txscript.SigHashDefault).
//
// Returns:
//   - A parsed Schnorr signature for the specified input.
//   - An error if the signing process fails or required inputs are missing.
func signTaprootScriptSpendInput(
	txToSign *wire.MsgTx,
	inputIdxToSign int,
	prevOutputs map[wire.OutPoint]*wire.TxOut,
	privKey *btcec.PrivateKey,
	tapLeaf txscript.TapLeaf,
	sigHashType txscript.SigHashType,
) (*schnorr.Signature, error) {
	if inputIdxToSign < 0 || inputIdxToSign >= len(txToSign.TxIn) {
		return nil, fmt.Errorf("input index %d out of range", inputIdxToSign)
	}

	inputFetcher := txscript.NewMultiPrevOutFetcher(nil)
	for outpoint, txOut := range prevOutputs {
		inputFetcher.AddPrevOut(outpoint, txOut)
	}

	inputToSign := txToSign.TxIn[inputIdxToSign]
	prevOut, ok := prevOutputs[inputToSign.PreviousOutPoint]
	if !ok {
		return nil, fmt.Errorf("missing prev output for input %d", inputIdxToSign)
	}

	sigHashes := txscript.NewTxSigHashes(txToSign, inputFetcher)

	sig, err := txscript.RawTxInTapscriptSignature(
		txToSign, sigHashes, inputIdxToSign, prevOut.Value,
		prevOut.PkScript, tapLeaf, sigHashType, privKey,
	)
	if err != nil {
		return nil, err
	}

	return schnorr.ParseSignature(sig)
}

// SignTxWithOneScriptSpendInputStrict signs transaction with one input coming
// from script spend output with provided script.
// It checks:
// - txToSign is not nil
// - txToSign has exactly one input
// - fundingTx is not nil
// - fundingTx has one output committing to the provided script
// - txToSign input is pointing to the correct output in fundingTx
func SignTxWithOneScriptSpendInputStrict(
	txToSign *wire.MsgTx,
	fundingTx *wire.MsgTx,
	fundingOutputIdx uint32,
	signedScriptPath []byte,
	privKey *btcec.PrivateKey,
) (*schnorr.Signature, error) {
	if err := checkTxBeforeSigning(txToSign, fundingTx, fundingOutputIdx); err != nil {
		return nil, fmt.Errorf("invalid tx: %w", err)
	}

	fundingOutput := fundingTx.TxOut[fundingOutputIdx]

	return SignTxWithOneScriptSpendInputFromScript(txToSign, fundingOutput, privKey, signedScriptPath)
}

// EncSignTxWithOneScriptSpendInputStrict is encrypted version of
// SignTxWithOneScriptSpendInputStrict with the output to be encrypted
// by an encryption key (adaptor signature)
func EncSignTxWithOneScriptSpendInputStrict(
	txToSign *wire.MsgTx,
	fundingTx *wire.MsgTx,
	fundingOutputIdx uint32,
	signedScriptPath []byte,
	privKey *btcec.PrivateKey,
	encKey *asig.EncryptionKey,
) (*asig.AdaptorSignature, error) {
	if err := checkTxBeforeSigning(txToSign, fundingTx, fundingOutputIdx); err != nil {
		return nil, fmt.Errorf("invalid tx: %w", err)
	}

	fundingOutput := fundingTx.TxOut[fundingOutputIdx]

	tapLeaf := txscript.NewBaseTapLeaf(signedScriptPath)

	inputFetcher := txscript.NewCannedPrevOutputFetcher(
		fundingOutput.PkScript,
		fundingOutput.Value,
	)

	sigHashes := txscript.NewTxSigHashes(txToSign, inputFetcher)

	sigHash, err := txscript.CalcTapscriptSignaturehash(
		sigHashes,
		txscript.SigHashDefault,
		txToSign,
		0,
		inputFetcher,
		tapLeaf)
	if err != nil {
		return nil, err
	}

	adaptorSig, err := asig.EncSign(privKey, encKey, sigHash)
	if err != nil {
		return nil, err
	}

	return adaptorSig, nil
}

func checkTxBeforeSigning(txToSign *wire.MsgTx, fundingTx *wire.MsgTx, fundingOutputIdx uint32) error {
	if txToSign == nil {
		return fmt.Errorf("tx to sign must not be nil")
	}

	if len(txToSign.TxIn) != 1 {
		return fmt.Errorf("tx to sign must have exactly one input")
	}

	if int(fundingOutputIdx) >= len(fundingTx.TxOut) {
		return fmt.Errorf("invalid funding output index %d, tx has %d outputs", fundingOutputIdx, len(fundingTx.TxOut))
	}

	fundingTxHash := fundingTx.TxHash()

	if !txToSign.TxIn[0].PreviousOutPoint.Hash.IsEqual(&fundingTxHash) {
		return fmt.Errorf("txToSign must input point to fundingTx")
	}

	if txToSign.TxIn[0].PreviousOutPoint.Index != fundingOutputIdx {
		return fmt.Errorf("txToSign inpunt index must point to output with provided script")
	}

	return nil
}

// VerifyTransactionSigWithOutput verifies that:
// - provided transaction has exactly one input
// - provided signature is valid schnorr BIP340 signature
// - provided signature is signing whole provided transaction	(SigHashDefault)
func VerifyTransactionSigWithOutput(
	transaction *wire.MsgTx,
	fundingOutput *wire.TxOut,
	script []byte,
	pubKey *btcec.PublicKey,
	signature []byte) error {
	if fundingOutput == nil {
		return fmt.Errorf("funding output must not be nil")
	}

	if transaction == nil {
		return fmt.Errorf("tx to verify not be nil")
	}

	if len(transaction.TxIn) != 1 {
		return fmt.Errorf("tx to sign must have exactly one input")
	}

	if pubKey == nil {
		return fmt.Errorf("public key must not be nil")
	}

	return verifyTaprootScriptSpendSignature(
		transaction,
		0,
		map[wire.OutPoint]*wire.TxOut{
			transaction.TxIn[0].PreviousOutPoint: fundingOutput,
		},
		txscript.NewBaseTapLeaf(script),
		pubKey,
		signature,
	)
}

// VerifyTransactionSigStkExp verifies that:
// - provided signature is valid schnorr BIP340 signature
// - provided signature is signing whole provided transaction	(SigHashDefault)
func VerifyTransactionSigStkExp(
	stkSpendTx *wire.MsgTx,
	fundingOutputSignedIdx0 *wire.TxOut,
	fundingOutputIdx1 *wire.TxOut,
	script []byte,
	pubKey *btcec.PublicKey,
	signatureOverPrevStkSpend []byte,
) error {
	if fundingOutputSignedIdx0 == nil {
		return fmt.Errorf("funding output for idx 0 must not be nil")
	}

	if fundingOutputIdx1 == nil {
		return fmt.Errorf("funding output for idx 1 must not be nil")
	}

	if stkSpendTx == nil {
		return fmt.Errorf("tx to verify not be nil")
	}

	if pubKey == nil {
		return fmt.Errorf("public key must not be nil")
	}

	if len(stkSpendTx.TxIn) != 2 {
		return fmt.Errorf("stake spend tx must have exactly two inputs")
	}

	return verifyTaprootScriptSpendSignature(
		stkSpendTx,
		0,
		map[wire.OutPoint]*wire.TxOut{
			stkSpendTx.TxIn[0].PreviousOutPoint: fundingOutputSignedIdx0,
			stkSpendTx.TxIn[1].PreviousOutPoint: fundingOutputIdx1,
		},
		txscript.NewBaseTapLeaf(script),
		pubKey,
		signatureOverPrevStkSpend,
	)
}

// verifyTaprootScriptSpendSignature verifies a Taproot script path signature for the given input index.
// It checks:
// - The signature is a valid BIP340 Schnorr signature.
// - The signature commits to the entire transaction with SigHashDefault.
// - The TapLeaf script is what was signed.
// - All prevOutputs must be supplied in full (for all inputs).
func verifyTaprootScriptSpendSignature(
	tx *wire.MsgTx,
	inputIdx int,
	prevOutputs map[wire.OutPoint]*wire.TxOut,
	tapLeaf txscript.TapLeaf,
	pubKey *btcec.PublicKey,
	signature []byte,
) error {
	if tx == nil {
		return fmt.Errorf("tx to verify must not be nil")
	}
	if pubKey == nil {
		return fmt.Errorf("public key must not be nil")
	}
	if inputIdx < 0 || inputIdx >= len(tx.TxIn) {
		return fmt.Errorf("input index %d out of bounds", inputIdx)
	}

	// Build fetcher with all previous outputs
	inputFetcher := txscript.NewMultiPrevOutFetcher(nil)
	for outpoint, txOut := range prevOutputs {
		inputFetcher.AddPrevOut(outpoint, txOut)
	}

	sigHashes := txscript.NewTxSigHashes(tx, inputFetcher)

	sigHash, err := txscript.CalcTapscriptSignaturehash(
		sigHashes,
		txscript.SigHashDefault,
		tx,
		inputIdx,
		inputFetcher,
		tapLeaf,
	)
	if err != nil {
		return err
	}

	parsedSig, err := schnorr.ParseSignature(signature)
	if err != nil {
		return err
	}

	if !parsedSig.Verify(sigHash, pubKey) {
		return fmt.Errorf("signature is not valid")
	}

	return nil
}

// EncVerifyTransactionSigWithOutput verifies that:
// - provided transaction has exactly one input
// - provided signature is valid adaptor signature
// - provided signature is signing whole provided transaction (SigHashDefault)
func EncVerifyTransactionSigWithOutput(
	transaction *wire.MsgTx,
	fundingOut *wire.TxOut,
	script []byte,
	pubKey *btcec.PublicKey,
	encKey *asig.EncryptionKey,
	signature *asig.AdaptorSignature,
) error {
	if transaction == nil {
		return fmt.Errorf("tx to verify not be nil")
	}

	if len(transaction.TxIn) != 1 {
		return fmt.Errorf("tx to sign must have exactly one input")
	}

	if pubKey == nil {
		return fmt.Errorf("public key must not be nil")
	}

	tapLeaf := txscript.NewBaseTapLeaf(script)

	inputFetcher := txscript.NewCannedPrevOutputFetcher(
		fundingOut.PkScript,
		fundingOut.Value,
	)

	sigHashes := txscript.NewTxSigHashes(transaction, inputFetcher)

	sigHash, err := txscript.CalcTapscriptSignaturehash(
		sigHashes, txscript.SigHashDefault, transaction, 0, inputFetcher, tapLeaf,
	)

	if err != nil {
		return err
	}

	return signature.EncVerify(pubKey, encKey, sigHash)
}

// SerializeTxOut serializes a wire.TxOut to a byte slice.
func SerializeTxOut(txOut *wire.TxOut) ([]byte, error) {
	var buf bytes.Buffer

	err := wire.WriteTxOut(&buf, 0, 0, txOut)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DeserializeTxOut deserializes a byte slice into a wire.TxOut.
func DeserializeTxOut(serializedBytes []byte) (*wire.TxOut, error) {
	var txOut wire.TxOut
	reader := bytes.NewReader(serializedBytes)

	err := wire.ReadTxOut(reader, 0, 0, &txOut)
	if err != nil {
		return nil, err
	}

	return &txOut, nil
}
