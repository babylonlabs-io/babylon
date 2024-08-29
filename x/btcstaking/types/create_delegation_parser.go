package types

import (
	"fmt"
	math "math"

	"github.com/babylonlabs-io/babylon/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcckpttypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ParsedPublicKey struct {
	*btcec.PublicKey
	*bbn.BIP340PubKey
}

func NewParsedPublicKey(key *bbn.BIP340PubKey) (*ParsedPublicKey, error) {
	if key == nil {
		return nil, fmt.Errorf("cannont parse nil *bbn.BIP340PubKey")
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
	info *btcckpttypes.TransactionInfo,
) (*ParsedProofOfInclusion, error) {
	if info == nil {
		return nil, fmt.Errorf("cannot parse nil *btcckpttypes.TransactionInfo")
	}

	if err := info.ValidateBasic(); err != nil {
		return nil, err
	}

	return &ParsedProofOfInclusion{
		HeaderHash: info.Key.Hash,
		Proof:      info.Proof,
		Index:      info.Key.Index,
	}, nil
}

type ParsedCreateDelegationMessage struct {
	StakerAddress              sdk.AccAddress
	StakingTx                  *ParsedBtcTransaction
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
}

// ParseCreateDelegationMessage parses a MsgCreateBTCDelegation message and performs some basic
// stateless checks:
// - unbonding transaction is a simple transfer
// - there is no duplicated keys in the finality provider key list
func ParseCreateDelegationMessage(msg *MsgCreateBTCDelegation) (*ParsedCreateDelegationMessage, error) {
	if msg == nil {
		return nil, fmt.Errorf("cannot parse nil MsgCreateBTCDelegation")
	}

	stakingTxProofOfInclusion, err := NewParsedProofOfInclusion(msg.StakingTx)

	if err != nil {
		return nil, fmt.Errorf("failed to parse staking tx proof of inclusion: %v", err)
	}

	// 1. Parse all transactions
	stakingTx, err := NewBtcTransaction(msg.StakingTx.Transaction)

	if err != nil {
		return nil, fmt.Errorf("failed to deserialize staking tx: %v", err)
	}

	stakingSlashingTx, err := NewBtcTransaction(msg.SlashingTx.MustMarshal())

	if err != nil {
		return nil, fmt.Errorf("failed to deserialize staking slashing tx: %v", err)
	}

	unbondingTx, err := NewBtcTransaction(msg.UnbondingTx)

	if err != nil {
		return nil, fmt.Errorf("failed to deserialize unbonding tx: %v", err)
	}

	if err := btcstaking.IsSimpleTransfer(unbondingTx.Transaction); err != nil {
		return nil, fmt.Errorf("unbonding tx is not a simple transfer: %v", err)
	}

	unbondingSlashingTx, err := NewBtcTransaction(msg.UnbondingSlashingTx.MustMarshal())

	if err != nil {
		return nil, fmt.Errorf("failed to deserialize unbonding slashing tx: %v", err)
	}

	// 2. Check all timelocks
	if msg.UnbondingTime > math.MaxUint16 {
		return nil, fmt.Errorf("unbonding time %d must be lower than %d", msg.UnbondingTime, math.MaxUint16)
	}

	if msg.StakingTime > math.MaxUint16 {
		return nil, fmt.Errorf("staking time %d must be lower than %d", msg.StakingTime, math.MaxUint16)
	}

	// 3. Parse staker address
	stakerAddr, err := sdk.AccAddressFromBech32(msg.StakerAddr)

	if err != nil {
		return nil, fmt.Errorf("invalid staker address %s: %v", msg.StakerAddr, err)
	}

	// 4. Parse proof of possession
	if msg.Pop == nil {
		return nil, fmt.Errorf("empty proof of possession")
	}

	if err := msg.Pop.ValidateBasic(); err != nil {
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

	if ExistsDup(fpPKs.PublicKeysBbnFormat) {
		return nil, ErrDuplicatedFp
	}

	// 7. Parse staker public key
	stakerPK, err := NewParsedPublicKey(msg.BtcPk)

	if err != nil {
		return nil, fmt.Errorf("failed to parse staker public key: %v", err)
	}

	// 8. Parse staking and unbonding value
	if msg.StakingValue < 0 {
		return nil, fmt.Errorf("staking value must be positive")
	}

	if msg.UnbondingValue < 0 {
		return nil, fmt.Errorf("unbonding value must be positive")
	}

	return &ParsedCreateDelegationMessage{
		StakerAddress:              stakerAddr,
		StakingTx:                  stakingTx,
		StakingTxProofOfInclusion:  stakingTxProofOfInclusion,
		StakingTime:                uint16(msg.StakingTime),
		StakingValue:               btcutil.Amount(msg.StakingValue),
		StakingSlashingTx:          stakingSlashingTx,
		StakerPK:                   stakerPK,
		StakerStakingSlashingTxSig: stakerStakingSlashingTxSig,
		UnbondingTx:                unbondingTx,
		UnbondingTime:              uint16(msg.UnbondingTime),
		UnbondingValue:             btcutil.Amount(msg.UnbondingValue),
		UnbondingSlashingTx:        unbondingSlashingTx,
		StakerUnbondingSlashingSig: stakerUnbondingSlashingSig,
		FinalityProviderKeys:       fpPKs,
		ParsedPop:                  msg.Pop,
	}, nil
}
