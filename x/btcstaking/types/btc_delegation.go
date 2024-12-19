package types

import (
	"bytes"
	"fmt"
	"math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"

	"github.com/babylonlabs-io/babylon/btcstaking"
	asig "github.com/babylonlabs-io/babylon/crypto/schnorr-adaptor-signature"
	bbn "github.com/babylonlabs-io/babylon/types"
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
	quorum uint32,
) (*asig.AdaptorSignature, error) {
	if !d.HasCovenantQuorums(quorum) {
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
	covenantQuorum uint32,
) BTCDelegationStatus {
	if d.IsUnbondedEarly() {
		return BTCDelegationStatus_UNBONDED
	}

	// we are still pending covenant quorum
	if !d.HasCovenantQuorums(covenantQuorum) {
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

	// if the endheight is lower than the btc height + unbonding time
	// the btc delegation should be considered expired
	if btcHeight+d.UnbondingTime > d.EndHeight {
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
func (d *BTCDelegation) VotingPower(btcHeight uint32, covenantQuorum uint32) uint64 {
	if d.GetStatus(btcHeight, covenantQuorum) != BTCDelegationStatus_ACTIVE {
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

func (d *BTCDelegation) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(d.StakerAddr); err != nil {
		return fmt.Errorf("invalid staker address: %s - %w", d.StakerAddr, err)
	}
	if d.BtcPk == nil {
		return fmt.Errorf("empty BTC public key")
	}
	if d.Pop == nil {
		return fmt.Errorf("empty proof of possession")
	}
	if len(d.FpBtcPkList) == 0 {
		return fmt.Errorf("empty list of finality provider PKs")
	}
	if ExistsDup(d.FpBtcPkList) {
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
		return err
	}

	return nil
}

// HasCovenantQuorums returns whether a BTC delegation has a quorum number of signatures
// from covenant members, including
// - adaptor signatures on slashing tx
// - Schnorr signatures on unbonding tx
// - adaptor signatrues on unbonding slashing tx
func (d *BTCDelegation) HasCovenantQuorums(quorum uint32) bool {
	return len(d.CovenantSigs) >= int(quorum) && d.BtcUndelegation.HasCovenantQuorums(quorum)
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
) {
	adaptorSigs := make([][]byte, 0, len(stakingSlashingSigs))
	for _, s := range stakingSlashingSigs {
		adaptorSigs = append(adaptorSigs, s.MustMarshal())
	}
	covSigs := &CovenantAdaptorSignatures{CovPk: covPk, AdaptorSigs: adaptorSigs}

	d.CovenantSigs = append(d.CovenantSigs, covSigs)
	// add unbonding sig and unbonding slashing adaptor sig
	d.BtcUndelegation.addCovenantSigs(covPk, unbondingSig, unbondingSlashingSigs)
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

func NewBTCDelegatorDelegationIndex() *BTCDelegatorDelegationIndex {
	return &BTCDelegatorDelegationIndex{
		StakingTxHashList: [][]byte{},
	}
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
