package types

import (
	"fmt"
	"math"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
)

type ParsedPublicKey struct {
	*btcec.PublicKey
	*bbn.BIP340PubKey
}

func NewParsedPublicKey(key *bbn.BIP340PubKey) (*ParsedPublicKey, error) {
	if key == nil {
		return nil, fmt.Errorf("cannot parse nil *bbn.BIP340PubKey")
	}
	pk, err := key.ToBTCPK()

	if err != nil {
		return nil, fmt.Errorf("failed to parse *bbn.BIP340PubKey: %w", err)
	}

	return &ParsedPublicKey{
		PublicKey:    pk,
		BIP340PubKey: key,
	}, nil
}

type ParsedBIP340Signature struct {
	*schnorr.Signature
	*bbn.BIP340Signature
}

func NewParsedBIP340Signature(sig *bbn.BIP340Signature) (*ParsedBIP340Signature, error) {
	if sig == nil {
		return nil, fmt.Errorf("cannot parse nil *bbn.BIP340Signature")
	}

	signature, err := sig.ToBTCSig()

	if err != nil {
		return nil, fmt.Errorf("failed to parse *bbn.BIP340Signature: %w", err)
	}

	return &ParsedBIP340Signature{
		Signature:       signature,
		BIP340Signature: sig,
	}, nil
}

type ParsedBtcTransaction struct {
	Transaction      *wire.MsgTx
	TransactionBytes []byte
}

func NewBtcTransaction(transactionBytes []byte) (*ParsedBtcTransaction, error) {
	tx, err := bbn.NewBTCTxFromBytes(transactionBytes)

	if err != nil {
		return nil, err
	}

	return &ParsedBtcTransaction{
		Transaction:      tx,
		TransactionBytes: transactionBytes,
	}, nil
}

type ParsedPublicKeyList struct {
	PublicKeys          []*btcec.PublicKey
	PublicKeysBbnFormat []bbn.BIP340PubKey
}

func NewParsedPublicKeyList(pks []bbn.BIP340PubKey) (*ParsedPublicKeyList, error) {
	if len(pks) == 0 {
		return nil, fmt.Errorf("cannot parse empty list of *bbn.BIP340PubKey")
	}

	parsedKeys, err := bbn.NewBTCPKsFromBIP340PKs(pks)

	if err != nil {
		return nil, fmt.Errorf("failed to parse list of *bbn.BIP340PubKey: %w", err)
	}

	return &ParsedPublicKeyList{
		PublicKeys:          parsedKeys,
		PublicKeysBbnFormat: pks,
	}, nil
}

type ParsedProofOfInclusion struct {
	HeaderHash *bbn.BTCHeaderHashBytes
	Proof      []byte
	Index      uint32
}

func NewParsedProofOfInclusion(
	ip *InclusionProof,
) (*ParsedProofOfInclusion, error) {
	if ip == nil {
		// this is allowed
		return nil, nil
	}

	if err := ip.ValidateBasic(); err != nil {
		return nil, err
	}

	return &ParsedProofOfInclusion{
		HeaderHash: ip.Key.Hash,
		Proof:      ip.Proof,
		Index:      ip.Key.Index,
	}, nil
}

type ParsedSignatureInfo struct {
	PublicKey *ParsedPublicKey
	Sig       *ParsedBIP340Signature
}

func (si *SignatureInfo) ValidateBasic() error {
	if si.Pk == nil {
		return fmt.Errorf("cannot parse nil *bbn.BIP340PubKey")
	}
	if si.Sig == nil {
		return fmt.Errorf("cannot parse nil *bbn.BIP340Signature")
	}

	return nil
}

func NewParsedSignatureInfo(si *SignatureInfo) (*ParsedSignatureInfo, error) {
	if si == nil {
		return nil, fmt.Errorf("cannot parse nil *SignatureInfo")
	}

	if err := si.ValidateBasic(); err != nil {
		return nil, err
	}

	pk, err := si.Pk.ToBTCPK()
	if err != nil {
		return nil, fmt.Errorf("failed to parse *bbn.BIP340PubKey: %w", err)
	}

	sig, err := si.Sig.ToBTCSig()
	if err != nil {
		return nil, fmt.Errorf("failed to parse *bbn.BIP340Signature: %w", err)
	}

	return &ParsedSignatureInfo{
		PublicKey: &ParsedPublicKey{pk, si.Pk},
		Sig:       &ParsedBIP340Signature{sig, si.Sig},
	}, nil
}

type ParsedCreateDelegationMessage struct {
	StakerAddress sdk.AccAddress
	StakingTx     *ParsedBtcTransaction
	// StakingTxInclusionProof is optional is and it is up to the caller to verify
	// whether it is present or not
	StakingTxProofOfInclusion  *ParsedProofOfInclusion
	StakingTime                uint16
	StakingValue               btcutil.Amount
	StakingSlashingTx          *ParsedBtcTransaction
	StakerPK                   *ParsedPublicKey
	StakerStakingSlashingTxSig *ParsedBIP340Signature
	UnbondingTx                *ParsedBtcTransaction
	UnbondingTime              uint16
	UnbondingValue             btcutil.Amount
	UnbondingSlashingTx        *ParsedBtcTransaction
	StakerUnbondingSlashingSig *ParsedBIP340Signature
	FinalityProviderKeys       *ParsedPublicKeyList
	ParsedPop                  *ProofOfPossessionBTC
	// StkExp is an optional field. If this filed is nil,
	// this BTC delegation is not an stake expansion. If
	// it is fulfilled it is a stake expansion and should
	// contain the necessary information to validate and
	// create the BTC delegation as a stake expansion.
	StkExp *ParsedCreateDelStkExp
	// ExtraStakerInfo is an optional field. if this field is nil,
	// this BTC delegation is not M-of-N multisig. else, it is an M-of-N multisig and
	// should contain the necessary information to validate.
	ExtraStakerInfo *ParsedAdditionalStakerInfo
}

type ParsedCreateDelStkExp struct {
	// PreviousActiveStkTxHash is the staking transaction hash of an
	// active BTC delegation that is being used as one of inputs to compose
	// this BTC Delegation, field is optional.
	PreviousActiveStkTxHash *chainhash.Hash
	// OtherFundingOutput that was used to pay for fees and optionally increase the
	// amount of BTC staked.
	OtherFundingOutput *wire.TxOut
	// FundingTxHash is the hash of the funding transaction that was used to pay for fees
	// and optionally increase the amount of BTC staked.
	FundingTxHash chainhash.Hash
	// FundingOutputIndex is the index of the OtherFundingOutput
	FundingOutputIndex uint32
}

type ParsedAdditionalStakerInfo struct {
	StakerBTCPkList             *ParsedPublicKeyList
	StakerQuorum                uint32
	StakerStakingSlashingSigs   []*ParsedSignatureInfo
	StakerUnbondingSlashingSigs []*ParsedSignatureInfo
}

func parseAdditionalStakerInfo(asi *AdditionalStakerInfo) (*ParsedAdditionalStakerInfo, error) {
	var (
		stakerStakingSlashingSigs, stakerUnbondingSlashingSigs []*ParsedSignatureInfo
	)

	stakerBTCPkList, err := NewParsedPublicKeyList(asi.StakerBtcPkList)
	if err != nil {
		return nil, err
	}

	for _, si := range asi.DelegatorSlashingSigs {
		parsedSi, err := NewParsedSignatureInfo(si)
		if err != nil {
			return nil, err
		}
		stakerStakingSlashingSigs = append(stakerStakingSlashingSigs, parsedSi)
	}

	for _, si := range asi.DelegatorUnbondingSlashingSigs {
		parsedSi, err := NewParsedSignatureInfo(si)
		if err != nil {
			return nil, err
		}
		stakerUnbondingSlashingSigs = append(stakerUnbondingSlashingSigs, parsedSi)
	}

	return &ParsedAdditionalStakerInfo{
		StakerBTCPkList:             stakerBTCPkList,
		StakerQuorum:                asi.StakerQuorum,
		StakerStakingSlashingSigs:   stakerStakingSlashingSigs,
		StakerUnbondingSlashingSigs: stakerUnbondingSlashingSigs,
	}, nil
}

// parseCreateDelegationMessage parses MsgCreateBTCDelegation message and performs some basic
// stateless checks:
// - unbonding transaction is a simple transfer
// - there is no duplicated keys in the finality provider key list
func parseCreateDelegationMessage(msg *MsgCreateBTCDelegation) (*ParsedCreateDelegationMessage, error) {
	// NOTE: stakingTxProofOfInclusion could be nil as we allow msg.StakingTxInclusionProof to be nil
	stakingTxProofOfInclusion, err := NewParsedProofOfInclusion(msg.GetStakingTxInclusionProof())
	if err != nil {
		return nil, fmt.Errorf("failed to parse staking tx proof of inclusion: %v", err)
	}

	// 1. Parse all transactions
	stakingTx, err := NewBtcTransaction(msg.GetStakingTx())
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize staking tx: %v", err)
	}

	stakingSlashingTx, err := NewBtcTransaction(msg.SlashingTx.MustMarshal())
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize staking slashing tx: %v", err)
	}

	unbondingTx, err := NewBtcTransaction(msg.GetUnbondingTx())
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize unbonding tx: %v", err)
	}

	unbondingSlashingTx, err := NewBtcTransaction(msg.UnbondingSlashingTx.MustMarshal())
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize unbonding slashing tx: %v", err)
	}

	// 2. Check all timelocks
	if msg.GetUnbondingTime() > math.MaxUint16 {
		return nil, fmt.Errorf("unbonding time %d must be lower than %d", msg.GetUnbondingTime(), math.MaxUint16)
	}

	if msg.GetStakingTime() > math.MaxUint16 {
		return nil, fmt.Errorf("staking time %d must be lower than %d", msg.GetStakingTime(), math.MaxUint16)
	}

	// 3. Parse staker address
	stakerAddr, err := sdk.AccAddressFromBech32(msg.GetStakerAddr())
	if err != nil {
		return nil, fmt.Errorf("invalid staker address %s: %v", msg.GetStakerAddr(), err)
	}

	// 4. Parse proof of possession
	if msg.GetPop() == nil {
		return nil, fmt.Errorf("empty proof of possession")
	}

	if err := msg.GetPop().ValidateBasic(); err != nil {
		return nil, err
	}

	// 5. Parse signatures for slashing transaction
	stakerStakingSlashingTxSig, err := NewParsedBIP340Signature(msg.DelegatorSlashingSig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse staker staking slashing signature: %v", err)
	}

	stakerUnbondingSlashingSig, err := NewParsedBIP340Signature(msg.DelegatorUnbondingSlashingSig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse staker unbonding slashing signature: %v", err)
	}

	// 6. Parse finality provider public keys and check for duplicates
	fpPKs, err := NewParsedPublicKeyList(msg.FpBtcPkList)
	if err != nil {
		return nil, fmt.Errorf("failed to parse finality provider public keys: %v", err)
	}

	duplicate, err := ExistsDup(fpPKs.PublicKeysBbnFormat)
	if err != nil {
		return nil, fmt.Errorf("error in FPs public keys: %v", err)
	}
	if duplicate {
		return nil, ErrDuplicatedFp
	}

	if len(fpPKs.PublicKeysBbnFormat) != 1 {
		return nil, ErrTooManyFpKeys
	}

	// 7. Parse staker public key
	stakerPK, err := NewParsedPublicKey(msg.BtcPk)
	if err != nil {
		return nil, fmt.Errorf("failed to parse staker public key: %v", err)
	}

	// 8. Parse staking and unbonding value
	if msg.GetStakingValue() < 0 {
		return nil, fmt.Errorf("staking value must be positive")
	}

	if msg.GetUnbondingValue() < 0 {
		return nil, fmt.Errorf("unbonding value must be positive")
	}

	// 9. Parse extra staker info
	var parsedExtraStakerInfo *ParsedAdditionalStakerInfo
	if msg.GetExtraStakerInfo() != nil {
		parsedExtraStakerInfo, err = parseAdditionalStakerInfo(msg.GetExtraStakerInfo())
		if err != nil {
			return nil, fmt.Errorf("failed to parse extra staker info: %v", err)
		}
	}

	return &ParsedCreateDelegationMessage{
		StakerAddress:              stakerAddr,
		StakingTx:                  stakingTx,
		StakingTxProofOfInclusion:  stakingTxProofOfInclusion,
		StakingTime:                uint16(msg.GetStakingTime()),
		StakingValue:               btcutil.Amount(msg.GetStakingValue()),
		StakingSlashingTx:          stakingSlashingTx,
		StakerPK:                   stakerPK,
		StakerStakingSlashingTxSig: stakerStakingSlashingTxSig,
		UnbondingTx:                unbondingTx,
		UnbondingTime:              uint16(msg.GetUnbondingTime()),
		UnbondingValue:             btcutil.Amount(msg.GetUnbondingValue()),
		UnbondingSlashingTx:        unbondingSlashingTx,
		StakerUnbondingSlashingSig: stakerUnbondingSlashingSig,
		FinalityProviderKeys:       fpPKs,
		ParsedPop:                  msg.GetPop(),
		ExtraStakerInfo:            parsedExtraStakerInfo,
	}, nil
}

// parseBtcExpandMessage parses MsgBtcStakeExpand message and performs some basic
// stateless checks:
// - unbonding transaction is a simple transfer
// - there is no duplicated keys in the finality provider key list
func parseBtcExpandMessage(msg *MsgBtcStakeExpand) (*ParsedCreateDelegationMessage, error) {
	// reuse parseCreateDelegationMessage cause MsgBtcStakeExpand has
	// same fields as MsgCreateBTCDelegation (plus 2 more related to stake expansion)
	parsed, err := parseCreateDelegationMessage(&MsgCreateBTCDelegation{
		StakerAddr:                    msg.StakerAddr,
		Pop:                           msg.Pop,
		BtcPk:                         msg.BtcPk,
		FpBtcPkList:                   msg.FpBtcPkList,
		StakingTime:                   msg.StakingTime,
		StakingValue:                  msg.StakingValue,
		StakingTx:                     msg.StakingTx,
		SlashingTx:                    msg.SlashingTx,
		DelegatorSlashingSig:          msg.DelegatorSlashingSig,
		UnbondingTime:                 msg.UnbondingTime,
		UnbondingTx:                   msg.UnbondingTx,
		UnbondingValue:                msg.UnbondingValue,
		UnbondingSlashingTx:           msg.UnbondingSlashingTx,
		DelegatorUnbondingSlashingSig: msg.DelegatorUnbondingSlashingSig,
		ExtraStakerInfo:               msg.ExtraStakerInfo,
	})
	if err != nil {
		return nil, err
	}
	stkExp, err := msg.GetStakeExpansion()
	if err != nil {
		return nil, err
	}
	parsed.StkExp = stkExp
	return parsed, nil
}

func (msg *ParsedCreateDelegationMessage) IsIncludedOnBTC() bool {
	return msg.StakingTxProofOfInclusion != nil
}

func (msg *ParsedCreateDelStkExp) SerializeOtherFundingOutput() ([]byte, error) {
	return btcstaking.SerializeTxOut(msg.OtherFundingOutput)
}
