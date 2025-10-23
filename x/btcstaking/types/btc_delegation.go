package types

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"

	"github.com/babylonlabs-io/babylon/v4/btcstaking"
	asig "github.com/babylonlabs-io/babylon/v4/crypto/schnorr-adaptor-signature"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
)

func NewBTCDelegationStatusFromString(statusStr string) (BTCDelegationStatus, error) {
	switch statusStr {
	case "pending":
		return BTCDelegationStatus_PENDING, nil
	case "verified":
		return BTCDelegationStatus_VERIFIED, nil
	case "active":
		return BTCDelegationStatus_ACTIVE, nil
	case "unbonded":
		return BTCDelegationStatus_UNBONDED, nil
	case "expired":
		return BTCDelegationStatus_EXPIRED, nil
	case "any":
		return BTCDelegationStatus_ANY, nil
	default:
		return -1, fmt.Errorf("invalid status string; should be one of {pending, verified, active, unbonded, any}")
	}
}

func (d *BTCDelegation) MustGetValidStakingTime() uint16 {
	if d.StakingTime > math.MaxUint16 {
		panic("invalid delegation in database")
	}

	return uint16(d.StakingTime)
}

func (d *BTCDelegation) HasInclusionProof() bool {
	return d.StartHeight > 0 && d.EndHeight > 0
}

// GetFpIdx returns the index of the finality provider in the list of finality providers
// that the BTC delegation is restaked to
func (d *BTCDelegation) GetFpIdx(fpBTCPK *bbn.BIP340PubKey) int {
	for i := 0; i < len(d.FpBtcPkList); i++ {
		if d.FpBtcPkList[i].Equals(fpBTCPK) {
			return i
		}
	}
	return -1
}

// Address returns the bech32 BTC staker address
func (d *BTCDelegation) Address() sdk.AccAddress {
	return sdk.MustAccAddressFromBech32(d.StakerAddr)
}

func (d *BTCDelegation) GetCovSlashingAdaptorSig(
	covBTCPK *bbn.BIP340PubKey,
	valIdx int,
	quorum, quorumPreviousStk uint32,
) (*asig.AdaptorSignature, error) {
	if !d.HasCovenantQuorums(quorum, quorumPreviousStk) {
		return nil, ErrInvalidDelegationState.Wrap("BTC delegation does not have a covenant quorum yet")
	}
	for _, covASigs := range d.CovenantSigs {
		if covASigs.CovPk.Equals(covBTCPK) {
			if valIdx >= len(covASigs.AdaptorSigs) {
				return nil, ErrFpNotFound.Wrap("validator index is out of scope")
			}
			sigBytes := covASigs.AdaptorSigs[valIdx]
			return asig.NewAdaptorSignatureFromBytes(sigBytes)
		}
	}

	return nil, ErrInvalidCovenantPK.Wrap("covenant PK is not found")
}

// IsUnbondedEarly returns whether the delegator has signed unbonding signature.
// Signing unbonding signature means the delegator wants to unbond early, and
// Babylon will consider this BTC delegation unbonded directly
func (d *BTCDelegation) IsUnbondedEarly() bool {
	return d.BtcUndelegation.DelegatorUnbondingInfo != nil
}

func (d *BTCDelegation) FinalityProviderKeys() []string {
	var fpPks = make([]string, len(d.FpBtcPkList))

	for i, fpPk := range d.FpBtcPkList {
		fpPks[i] = fpPk.MarshalHex()
	}

	return fpPks
}

// GetStatus returns the status of the BTC Delegation based on BTC height,
// unbonding time, and covenant quorum
// Pending: the BTC height is in the range of d's [startHeight, endHeight-unbondingTime]
// and the delegation does not have covenant signatures
// Active: the BTC height is in the range of d's [startHeight, endHeight-unbondingTime]
// and the delegation has quorum number of signatures over slashing tx,
// unbonding tx, and slashing unbonding tx from covenant committee
// Unbonded: the BTC height is larger than `endHeight-unbondingTime` or the
// BTC delegation has received a signature on unbonding tx from the delegator
func (d *BTCDelegation) GetStatus(
	btcHeight uint32,
	covenantQuorum, quorumPreviousStk uint32,
) BTCDelegationStatus {
	if d.IsUnbondedEarly() {
		return BTCDelegationStatus_UNBONDED
	}

	// we are still pending covenant quorum
	if !d.HasCovenantQuorums(covenantQuorum, quorumPreviousStk) {
		return BTCDelegationStatus_PENDING
	}

	// we are still pending activation by inclusion proof
	if !d.HasInclusionProof() {
		// staking tx has not been included in a block yet
		return BTCDelegationStatus_VERIFIED
	}

	// At this point we already have covenant quorum and inclusion proof,
	// we can check the status based on the BTC height
	if btcHeight < d.StartHeight {
		// staking tx's timelock has not begun, or is less than unbonding time BTC
		// blocks left
		return BTCDelegationStatus_UNBONDED
	}

	// if the endheight is not higher than the btc height + unbonding time
	// the btc delegation should be considered expired
	if btcHeight+d.UnbondingTime >= d.EndHeight {
		// It is needed to use ">=" instead of just ">"
		// to avoid processing both events at the same time
		// Unbonding + Expired. If the unbonding request
		// was about to be processed in the same block
		// as the expired event.
		return BTCDelegationStatus_EXPIRED
	}

	// - we have covenant quorum
	// - we have inclusion proof
	// - we are not unbonded early
	// - we are not expired
	return BTCDelegationStatus_ACTIVE
}

// VotingPower returns the voting power of the BTC delegation at a given BTC height
// The BTC delegation d has voting power iff it is active.
func (d *BTCDelegation) VotingPower(btcHeight uint32, covenantQuorum, quorumPreviousStk uint32) uint64 {
	if d.GetStatus(btcHeight, covenantQuorum, quorumPreviousStk) != BTCDelegationStatus_ACTIVE {
		return 0
	}
	return d.GetTotalSat()
}

func (d *BTCDelegation) GetStakingTxHash() (chainhash.Hash, error) {
	parsed, err := bbn.NewBTCTxFromBytes(d.StakingTx)

	if err != nil {
		return chainhash.Hash{}, err
	}

	return parsed.TxHash(), nil
}

func (d *BTCDelegation) MustGetStakingTxHash() chainhash.Hash {
	txHash, err := d.GetStakingTxHash()

	if err != nil {
		panic(err)
	}

	return txHash
}

func (d *BTCDelegation) MustGetStakingTx() *wire.MsgTx {
	stakingTx, err := bbn.NewBTCTxFromBytes(d.StakingTx)

	if err != nil {
		panic(err)
	}

	return stakingTx
}

func (d *BTCDelegation) MustGetUnbondingTx() *wire.MsgTx {
	unbondingTx, err := bbn.NewBTCTxFromBytes(d.BtcUndelegation.UnbondingTx)

	if err != nil {
		panic(err)
	}

	return unbondingTx
}

func (d *BTCDelegation) StakeExpansionTxHash() (*chainhash.Hash, error) {
	if !d.IsStakeExpansion() {
		return nil, errors.New("stake expansion not found. This is not a stake expansion delegation")
	}
	return d.StkExp.StakeExpansionTxHash()
}

func (d *BTCDelegation) MustGetStakeExpansionTxHash() *chainhash.Hash {
	txHash, err := d.StakeExpansionTxHash()
	if err != nil {
		panic(fmt.Errorf("failed to parse %+v as chain hash", d.StkExp.PreviousStakingTxHash))
	}
	return txHash
}

func (d *BTCDelegation) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(d.StakerAddr); err != nil {
		return fmt.Errorf("invalid staker address: %s - %w", d.StakerAddr, err)
	}
	if d.BtcPk == nil {
		return fmt.Errorf("empty BTC public key")
	}
	if _, err := d.BtcPk.ToBTCPK(); err != nil {
		return fmt.Errorf("BtcPk is not correctly formatted: %w", err)
	}
	if d.Pop == nil {
		return fmt.Errorf("empty proof of possession")
	}
	if err := d.Pop.ValidateBasic(); err != nil {
		return err
	}
	if len(d.FpBtcPkList) == 0 {
		return fmt.Errorf("empty list of finality provider PKs")
	}
	duplicate, err := ExistsDup(d.FpBtcPkList)
	if err != nil {
		return fmt.Errorf("list of finality provider PKs has an error: %w", err)
	}
	if duplicate {
		return fmt.Errorf("list of finality provider PKs has duplication")
	}
	if d.StakingTx == nil {
		return fmt.Errorf("empty staking tx")
	}
	if d.SlashingTx == nil {
		return fmt.Errorf("empty slashing tx")
	}
	if d.DelegatorSig == nil {
		return fmt.Errorf("empty delegator signature")
	}

	// ensure staking tx is correctly formatted
	if _, err := bbn.NewBTCTxFromBytes(d.StakingTx); err != nil {
		return fmt.Errorf("failed to deserialize staking tx: %v", err)
	}

	// ensure slashing tx is correctly formatted
	if _, err := bbn.NewBTCTxFromBytes(d.SlashingTx.MustMarshal()); err != nil {
		return fmt.Errorf("failed to deserialize slashing tx: %v", err)
	}

	// Check all timelocks
	if d.UnbondingTime > math.MaxUint16 {
		return fmt.Errorf("unbonding time %d must be lower than %d", d.UnbondingTime, math.MaxUint16)
	}

	if d.StakingTime > math.MaxUint16 {
		return fmt.Errorf("staking time %d must be lower than %d", d.StakingTime, math.MaxUint16)
	}

	if d.IsStakeExpansion() {
		if err := d.StkExp.Validate(); err != nil {
			return err
		}
	}

	if d.IsMultisigBtcDel() {
		if err := d.MultisigInfo.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// HasCovenantQuorums returns whether a BTC delegation has a quorum number of signatures
// from covenant members, including
// - adaptor signatures on slashing tx
// - Schnorr signatures on unbonding tx
// - adaptor signatrues on unbonding slashing tx
func (d *BTCDelegation) HasCovenantQuorums(quorum, quorumPreviousStk uint32) bool {
	hasQuorum := len(d.CovenantSigs) >= int(quorum) && d.BtcUndelegation.HasCovenantQuorums(quorum)
	if d.IsStakeExpansion() {
		return hasQuorum && d.StkExp.HasCovenantQuorums(quorumPreviousStk)
	}
	return hasQuorum
}

// IsSignedByCovMember checks whether the given covenant PK has signed the delegation
func (d *BTCDelegation) IsSignedByCovMember(covPk *bbn.BIP340PubKey) bool {
	for _, sigInfo := range d.CovenantSigs {
		if covPk.Equals(sigInfo.CovPk) {
			return true
		}
	}

	return false
}

// AddCovenantSigs adds signatures on the slashing tx from the given
// covenant, where each signature is an adaptor signature encrypted by
// each finality provider's PK this BTC delegation restakes to
// It is up to the caller to ensure that given adaptor signatures are valid or
// that they were not added before
func (d *BTCDelegation) AddCovenantSigs(
	covPk *bbn.BIP340PubKey,
	stakingSlashingSigs []asig.AdaptorSignature,
	unbondingSig *bbn.BIP340Signature,
	unbondingSlashingSigs []asig.AdaptorSignature,
	stkExpSig *bbn.BIP340Signature,
) {
	adaptorSigs := make([][]byte, 0, len(stakingSlashingSigs))
	for _, s := range stakingSlashingSigs {
		adaptorSigs = append(adaptorSigs, s.MustMarshal())
	}
	covSigs := &CovenantAdaptorSignatures{CovPk: covPk, AdaptorSigs: adaptorSigs}

	d.CovenantSigs = append(d.CovenantSigs, covSigs)
	// add unbonding sig and unbonding slashing adaptor sig
	d.BtcUndelegation.addCovenantSigs(covPk, unbondingSig, unbondingSlashingSigs)

	if d.IsStakeExpansion() {
		d.StkExp.AddCovenantSigs(covPk, stkExpSig)
	}
}

// GetStakingInfo returns the staking info of the BTC delegation
// the staking info can be used for constructing witness of slashing tx
// with access to a finality provider's SK
func (d *BTCDelegation) GetStakingInfo(bsParams *Params, btcNet *chaincfg.Params) (*btcstaking.StakingInfo, error) {
	fpBtcPkList, err := bbn.NewBTCPKsFromBIP340PKs(d.FpBtcPkList)
	if err != nil {
		return nil, fmt.Errorf("failed to convert finality provider pks to BTC pks %v", err)
	}
	covenantBtcPkList, err := bbn.NewBTCPKsFromBIP340PKs(bsParams.CovenantPks)
	if err != nil {
		return nil, fmt.Errorf("failed to convert covenant pks to BTC pks %v", err)
	}
	stakingInfo, err := btcstaking.BuildStakingInfo(
		d.BtcPk.MustToBTCPK(),
		fpBtcPkList,
		covenantBtcPkList,
		bsParams.CovenantQuorum,
		d.MustGetValidStakingTime(),
		btcutil.Amount(d.TotalSat),
		btcNet,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create BTC staking info: %v", err)
	}
	return stakingInfo, nil
}

func (d *BTCDelegation) SignUnbondingTx(bsParams *Params, btcNet *chaincfg.Params, sk *btcec.PrivateKey) (*schnorr.Signature, error) {
	stakingTx, err := bbn.NewBTCTxFromBytes(d.StakingTx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse staking transaction: %v", err)
	}
	unbondingTx, err := bbn.NewBTCTxFromBytes(d.BtcUndelegation.UnbondingTx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse unbonding transaction: %v", err)
	}
	stakingInfo, err := d.GetStakingInfo(bsParams, btcNet)
	if err != nil {
		return nil, err
	}
	unbondingPath, err := stakingInfo.UnbondingPathSpendInfo()
	if err != nil {
		return nil, err
	}

	sig, err := btcstaking.SignTxWithOneScriptSpendInputStrict(
		unbondingTx,
		stakingTx,
		d.StakingOutputIdx,
		unbondingPath.GetPkScriptPath(),
		sk,
	)
	if err != nil {
		return nil, err
	}
	return sig, nil
}

// GetUnbondingInfo returns the unbonding info of the BTC delegation
// the unbonding info can be used for constructing witness of unbonding slashing
// tx with access to a finality provider's SK
func (d *BTCDelegation) GetUnbondingInfo(bsParams *Params, btcNet *chaincfg.Params) (*btcstaking.UnbondingInfo, error) {
	fpBtcPkList, err := bbn.NewBTCPKsFromBIP340PKs(d.FpBtcPkList)
	if err != nil {
		return nil, fmt.Errorf("failed to convert finality provider pks to BTC pks: %v", err)
	}

	covenantBtcPkList, err := bbn.NewBTCPKsFromBIP340PKs(bsParams.CovenantPks)
	if err != nil {
		return nil, fmt.Errorf("failed to convert covenant pks to BTC pks: %v", err)
	}
	unbondingTx, err := bbn.NewBTCTxFromBytes(d.BtcUndelegation.UnbondingTx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse unbonding transaction: %v", err)
	}

	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		d.BtcPk.MustToBTCPK(),
		fpBtcPkList,
		covenantBtcPkList,
		bsParams.CovenantQuorum,
		uint16(d.GetUnbondingTime()),
		btcutil.Amount(unbondingTx.TxOut[0].Value),
		btcNet,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create BTC staking info: %v", err)
	}

	return unbondingInfo, nil
}

// findFPIdx returns the index of the given finality provider
// among all restaked finality providers
func (d *BTCDelegation) findFPIdx(fpBTCPK *bbn.BIP340PubKey) (int, error) {
	for i, pk := range d.FpBtcPkList {
		if pk.Equals(fpBTCPK) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("the given finality provider's PK is not found in the BTC delegation")
}

// BuildSlashingTxWithWitness uses the given finality provider's SK to complete
// the signatures on the slashing tx, such that the slashing tx obtains full
// witness and can be submitted to Bitcoin.
// This happens after the finality provider is slashed and its SK is extracted.
func (d *BTCDelegation) BuildSlashingTxWithWitness(bsParams *Params, btcNet *chaincfg.Params, fpSK *btcec.PrivateKey) (*wire.MsgTx, error) {
	stakingMsgTx, err := bbn.NewBTCTxFromBytes(d.StakingTx)
	if err != nil {
		return nil, fmt.Errorf("failed to convert a Babylon staking tx to wire.MsgTx: %w", err)
	}

	// get staking info
	stakingInfo, err := d.GetStakingInfo(bsParams, btcNet)
	if err != nil {
		return nil, fmt.Errorf("could not create BTC staking info: %v", err)
	}
	slashingSpendInfo, err := stakingInfo.SlashingPathSpendInfo()
	if err != nil {
		return nil, fmt.Errorf("could not get slashing spend info: %v", err)
	}

	// get the list of covenant signatures encrypted by the given finality provider's PK
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(fpSK.PubKey())
	fpIdx, err := d.findFPIdx(fpBTCPK)
	if err != nil {
		return nil, err
	}
	covAdaptorSigs, err := GetOrderedCovenantSignatures(fpIdx, d.CovenantSigs, bsParams)
	if err != nil {
		return nil, fmt.Errorf("failed to get ordered covenant adaptor signatures: %w", err)
	}

	// assemble witness for slashing tx
	slashingMsgTxWithWitness, err := d.SlashingTx.BuildSlashingTxWithWitness(
		fpSK,
		d.FpBtcPkList,
		stakingMsgTx,
		d.StakingOutputIdx,
		d.DelegatorSig,
		covAdaptorSigs,
		bsParams.CovenantQuorum,
		slashingSpendInfo,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to build witness for BTC delegation of %s under finality provider %s: %v",
			d.BtcPk.MarshalHex(),
			bbn.NewBIP340PubKeyFromBTCPK(fpSK.PubKey()).MarshalHex(),
			err,
		)
	}

	return slashingMsgTxWithWitness, nil
}

func (d *BTCDelegation) BuildUnbondingSlashingTxWithWitness(bsParams *Params, btcNet *chaincfg.Params, fpSK *btcec.PrivateKey) (*wire.MsgTx, error) {
	unbondingMsgTx, err := bbn.NewBTCTxFromBytes(d.BtcUndelegation.UnbondingTx)
	if err != nil {
		return nil, fmt.Errorf("failed to convert a Babylon unbonding tx to wire.MsgTx: %w", err)
	}

	// get unbonding info
	unbondingInfo, err := d.GetUnbondingInfo(bsParams, btcNet)
	if err != nil {
		return nil, fmt.Errorf("could not create BTC unbonding info: %v", err)
	}
	slashingSpendInfo, err := unbondingInfo.SlashingPathSpendInfo()
	if err != nil {
		return nil, fmt.Errorf("could not get unbonding slashing spend info: %v", err)
	}

	// get the list of covenant signatures encrypted by the given finality provider's PK
	fpPK := fpSK.PubKey()
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(fpPK)
	fpIdx, err := d.findFPIdx(fpBTCPK)
	if err != nil {
		return nil, err
	}
	covAdaptorSigs, err := GetOrderedCovenantSignatures(fpIdx, d.BtcUndelegation.CovenantSlashingSigs, bsParams)
	if err != nil {
		return nil, fmt.Errorf("failed to get ordered covenant adaptor signatures: %w", err)
	}

	// assemble witness for unbonding slashing tx
	slashingMsgTxWithWitness, err := d.BtcUndelegation.SlashingTx.BuildSlashingTxWithWitness(
		fpSK,
		d.FpBtcPkList,
		unbondingMsgTx,
		0,
		d.BtcUndelegation.DelegatorSlashingSig,
		covAdaptorSigs,
		bsParams.CovenantQuorum,
		slashingSpendInfo,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to build witness for unbonding BTC delegation %s under finality provider %s: %v",
			d.BtcPk.MarshalHex(),
			bbn.NewBIP340PubKeyFromBTCPK(fpSK.PubKey()).MarshalHex(),
			err,
		)
	}

	return slashingMsgTxWithWitness, nil
}

// IsStakeExpansion returns true if the BTC delegation was created
// using a previous staking transaction
func (d *BTCDelegation) IsStakeExpansion() bool {
	return d.StkExp != nil
}

// IsMultisigBtcDel return true if the BTC delegation contains `MultisigInfo`
func (d *BTCDelegation) IsMultisigBtcDel() bool {
	return d.MultisigInfo != nil
}

func (s *StakeExpansion) AddCovenantSigs(
	covPk *bbn.BIP340PubKey,
	stkExpSig *bbn.BIP340Signature,
) {
	prevStkCovSigs := &SignatureInfo{Pk: covPk, Sig: stkExpSig}
	s.PreviousStkCovenantSigs = append(s.PreviousStkCovenantSigs, prevStkCovSigs)
}

func (s *StakeExpansion) StakeExpansionTxHash() (*chainhash.Hash, error) {
	return chainhash.NewHash(s.PreviousStakingTxHash)
}

func (s *StakeExpansion) FundingTxOut() (*wire.TxOut, error) {
	return btcstaking.DeserializeTxOut(s.OtherFundingTxOut)
}

func (s *StakeExpansion) HasCovenantQuorums(quorumPreviousStk uint32) bool {
	return len(s.PreviousStkCovenantSigs) >= int(quorumPreviousStk)
}

func (s *StakeExpansion) ToResponse() *StakeExpansionResponse {
	previousStk, err := s.StakeExpansionTxHash()
	if err != nil {
		return nil
	}

	otherFundingTxOutHex := hex.EncodeToString(s.OtherFundingTxOut)

	return &StakeExpansionResponse{
		PreviousStakingTxHashHex: previousStk.String(),
		OtherFundingTxOutHex:     otherFundingTxOutHex,
		PreviousStkCovenantSigs:  s.PreviousStkCovenantSigs,
	}
}

func (s *StakeExpansion) Validate() error {
	if len(s.PreviousStakingTxHash) == 0 {
		return errorsmod.Wrapf(ErrInvalidStakeExpansion, "PreviousStakingTxHash is required")
	}

	if _, err := s.StakeExpansionTxHash(); err != nil {
		return errorsmod.Wrap(ErrInvalidStakeExpansion, err.Error())
	}

	if len(s.OtherFundingTxOut) == 0 {
		return errorsmod.Wrapf(ErrInvalidStakeExpansion, "OtherFundingTxOut is required")
	}

	if _, err := s.FundingTxOut(); err != nil {
		return errorsmod.Wrap(ErrInvalidStakeExpansion, err.Error())
	}
	for i, sig := range s.PreviousStkCovenantSigs {
		if sig == nil {
			return errorsmod.Wrapf(ErrInvalidStakeExpansion, "PreviousStkCovenantSigs[%d] is nil", i)
		}
		if err := sig.Validate(); err != nil {
			return errorsmod.Wrapf(ErrInvalidStakeExpansion, "invalid signature at index %d: %v", i, err)
		}
	}

	return nil
}

// IsSignedByCovMember checks whether the given covenant PK has signed the delegation
func (s *StakeExpansion) IsSignedByCovMember(covPk *bbn.BIP340PubKey) bool {
	for _, sigInfo := range s.PreviousStkCovenantSigs {
		if covPk.Equals(sigInfo.Pk) {
			return true
		}
	}

	return false
}

func (si *SignatureInfo) Validate() error {
	if si.Pk == nil {
		return fmt.Errorf("public key is nil")
	}
	if si.Pk.Size() != schnorr.PubKeyBytesLen {
		return fmt.Errorf("public key is invalid")
	}

	if si.Sig == nil {
		return fmt.Errorf("signature is nil")
	}
	if si.Sig.Size() != schnorr.SignatureSize {
		return fmt.Errorf("signature is invalid")
	}

	return nil
}

func NewBTCDelegatorDelegationIndex() *BTCDelegatorDelegationIndex {
	return &BTCDelegatorDelegationIndex{
		StakingTxHashList: [][]byte{},
	}
}

func (i *BTCDelegatorDelegationIndex) Validate() error {
	for _, bz := range i.StakingTxHashList {
		// NewHash validates hash size
		if _, err := chainhash.NewHash(bz); err != nil {
			return err
		}
	}
	return nil
}

func (i *BTCDelegatorDelegationIndex) Has(stakingTxHash chainhash.Hash) bool {
	for _, hash := range i.StakingTxHashList {
		if bytes.Equal(stakingTxHash[:], hash) {
			return true
		}
	}
	return false
}

func (i *BTCDelegatorDelegationIndex) Add(stakingTxHash chainhash.Hash) error {
	// ensure staking tx hash is not duplicated
	for _, hash := range i.StakingTxHashList {
		if bytes.Equal(stakingTxHash[:], hash) {
			return fmt.Errorf("the given stakingTxHash %s is duplicated", stakingTxHash.String())
		}
	}
	// add
	i.StakingTxHashList = append(i.StakingTxHashList, stakingTxHash[:])

	return nil
}

func (a *AdditionalStakerInfo) ToResponse() *AdditionalStakerInfoResponse {
	return &AdditionalStakerInfoResponse{
		StakerBtcPkList:                a.StakerBtcPkList,
		StakerQuorum:                   a.StakerQuorum,
		DelegatorSlashingSigs:          a.DelegatorSlashingSigs,
		DelegatorUnbondingSlashingSigs: a.DelegatorUnbondingSlashingSigs,
	}
}

func (a *AdditionalStakerInfo) Validate() error {
	if len(a.StakerBtcPkList) == 0 {
		return fmt.Errorf("length of the stakerBtcPkList is zero")
	}

	if a.StakerQuorum < 0 {
		return fmt.Errorf("stakerQuorum is negative")
	}

	if len(a.DelegatorSlashingSigs) == 0 {
		return fmt.Errorf("length of the delegatorSlashingSigs is zero")
	}

	if len(a.DelegatorUnbondingSlashingSigs) == 0 {
		return fmt.Errorf("length of the delegatorUnbondingSlashingSigs is zero")
	}

	for i, sig := range a.DelegatorSlashingSigs {
		if sig == nil {
			return errorsmod.Wrapf(ErrInvalidMultisigInfo, "DelegatorSlashingSigs[%d] is nil", i)
		}
		if err := sig.Validate(); err != nil {
			return errorsmod.Wrapf(ErrInvalidMultisigInfo, "invalid signature at index %d: %v", i, err)
		}
	}

	for i, sig := range a.DelegatorUnbondingSlashingSigs {
		if sig == nil {
			return errorsmod.Wrapf(ErrInvalidMultisigInfo, "DelegatorUnbondingSlashingSigs[%d] is nil", i)
		}
		if err := sig.Validate(); err != nil {
			return errorsmod.Wrapf(ErrInvalidMultisigInfo, "invalid signature at index %d: %v", i, err)
		}
	}

	return nil
}
