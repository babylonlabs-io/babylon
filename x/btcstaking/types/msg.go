package types

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// ensure that these message types implement the sdk.Msg interface
var (
	_ sdk.Msg = &MsgUpdateParams{}
	_ sdk.Msg = &MsgCreateFinalityProvider{}
	_ sdk.Msg = &MsgEditFinalityProvider{}
	_ sdk.Msg = &MsgCreateBTCDelegation{}
	_ sdk.Msg = &MsgAddCovenantSigs{}
	_ sdk.Msg = &MsgBTCUndelegate{}
	_ sdk.Msg = &MsgAddBTCDelegationInclusionProof{}
	_ sdk.Msg = &MsgSelectiveSlashingEvidence{}
	// Ensure msgs implement ValidateBasic
	_ sdk.HasValidateBasic = &MsgUpdateParams{}
	_ sdk.HasValidateBasic = &MsgCreateFinalityProvider{}
	_ sdk.HasValidateBasic = &MsgEditFinalityProvider{}
	_ sdk.HasValidateBasic = &MsgCreateBTCDelegation{}
	_ sdk.HasValidateBasic = &MsgAddCovenantSigs{}
	_ sdk.HasValidateBasic = &MsgBTCUndelegate{}
	_ sdk.HasValidateBasic = &MsgAddBTCDelegationInclusionProof{}
	_ sdk.HasValidateBasic = &MsgSelectiveSlashingEvidence{}
)

func (m MsgUpdateParams) ValidateBasic() error {
	return m.Params.Validate()
}

func (m *MsgCreateFinalityProvider) ValidateBasic() error {
	if err := m.Commission.Validate(); err != nil {
		return err
	}
	if err := validateDescription(m.Description); err != nil {
		return err
	}
	if m.BtcPk == nil {
		return fmt.Errorf("empty BTC public key")
	}
	if _, err := m.BtcPk.ToBTCPK(); err != nil {
		return fmt.Errorf("invalid BTC public key: %v", err)
	}
	if m.Pop == nil {
		return fmt.Errorf("empty proof of possession")
	}
	if _, err := sdk.AccAddressFromBech32(m.Addr); err != nil {
		return fmt.Errorf("invalid FP addr: %s - %v", m.Addr, err)
	}
	return m.Pop.ValidateBasic()
}

func (m *MsgEditFinalityProvider) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Addr); err != nil {
		return fmt.Errorf("invalid FP addr: %s - %v", m.Addr, err)
	}
	if m.Commission != nil {
		if m.Commission.IsNegative() {
			return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "commission rate must be between 0 and 1 (inclusive). Got negative value")
		}
		if m.Commission.GT(math.LegacyOneDec()) {
			return ErrCommissionGTMaxRate
		}
	}
	if m.Description == nil {
		return fmt.Errorf("empty description")
	}
	if len(m.Description.Moniker) == 0 {
		return fmt.Errorf("empty moniker")
	}
	if _, err := m.Description.EnsureLength(); err != nil {
		return err
	}
	if len(m.BtcPk) != bbn.BIP340PubKeyLen {
		return fmt.Errorf("malformed BTC PK")
	}
	if _, err := bbn.NewBIP340PubKey(m.BtcPk); err != nil {
		return err
	}

	return nil
}

// GetSlashingTx returns the slashing tx
func (m *MsgCreateBTCDelegation) GetSlashingTx() *BTCSlashingTx {
	return m.SlashingTx
}

// GetUnbondingSlashingTx returns the unbond slashing tx
func (m *MsgCreateBTCDelegation) GetUnbondingSlashingTx() *BTCSlashingTx {
	return m.UnbondingSlashingTx
}

// GetDelegatorSlashingSig returns the slashing signature
func (m *MsgCreateBTCDelegation) GetDelegatorSlashingSig() *bbn.BIP340Signature {
	return m.DelegatorSlashingSig
}

// GetDelegatorUnbondingSlashingSig returns the unbonding slashing signature
func (m *MsgCreateBTCDelegation) GetDelegatorUnbondingSlashingSig() *bbn.BIP340Signature {
	return m.DelegatorUnbondingSlashingSig
}

// GetFpBtcPkList returns the list of FP BTC PK
func (m *MsgCreateBTCDelegation) GetFpBtcPkList() []bbn.BIP340PubKey {
	return m.FpBtcPkList
}

// GetBtcPk returns the btc delegator BTC PK
func (m *MsgCreateBTCDelegation) GetBtcPk() *bbn.BIP340PubKey {
	return m.BtcPk
}

// GetStakeExpansion returns the stake expansion
func (m *MsgCreateBTCDelegation) GetStakeExpansion() (*ParsedCreateDelStkExp, error) {
	return nil, nil
}

// ToParsed returns a parsed ParsedCreateDelegationMessage or error if it fails
func (m *MsgCreateBTCDelegation) ToParsed() (*ParsedCreateDelegationMessage, error) {
	if m == nil {
		return nil, fmt.Errorf("cannot parse nil MsgCreateBTCDelegation")
	}

	return ParseCreateDelegationMessage(m)
}

func (m *MsgCreateBTCDelegation) ValidateBasic() error {
	if _, err := ParseCreateDelegationMessage(m); err != nil {
		return err
	}

	return nil
}

// GetSlashingTx returns the slashing tx
func (m *MsgBtcStakeExpand) GetSlashingTx() *BTCSlashingTx {
	return m.SlashingTx
}

// GetUnbondingSlashingTx returns the unbond slashing tx
func (m *MsgBtcStakeExpand) GetUnbondingSlashingTx() *BTCSlashingTx {
	return m.UnbondingSlashingTx
}

// GetDelegatorSlashingSig returns the slashing signature
func (m *MsgBtcStakeExpand) GetDelegatorSlashingSig() *bbn.BIP340Signature {
	return m.DelegatorSlashingSig
}

// GetDelegatorUnbondingSlashingSig returns the unbonding slashing signature
func (m *MsgBtcStakeExpand) GetDelegatorUnbondingSlashingSig() *bbn.BIP340Signature {
	return m.DelegatorUnbondingSlashingSig
}

// GetFpBtcPkList returns the list of FP BTC PK
func (m *MsgBtcStakeExpand) GetFpBtcPkList() []bbn.BIP340PubKey {
	return m.FpBtcPkList
}

// GetBtcPk returns the btc delegator BTC PK
func (m *MsgBtcStakeExpand) GetBtcPk() *bbn.BIP340PubKey {
	return m.BtcPk
}

// GetStakeExpansion returns the parsed stake expansion
func (m *MsgBtcStakeExpand) GetStakeExpansion() (*ParsedCreateDelStkExp, error) {
	previousActiveStkTxHash, err := chainhash.NewHashFromStr(m.PreviousStakingTxHash)
	if err != nil {
		return nil, err
	}

	stkExpandTx, err := bbn.NewBTCTxFromBytes(m.StakingTx)
	if err != nil {
		return nil, err
	}

	fundingTx, err := bbn.NewBTCTxFromBytes(m.FundingTx)
	if err != nil {
		return nil, err
	}

	if len(stkExpandTx.TxIn) != 2 {
		return nil, fmt.Errorf("stake expansion must have 2 inputs (TxIn)")
	}

	if !stkExpandTx.TxIn[0].PreviousOutPoint.Hash.IsEqual(previousActiveStkTxHash) {
		return nil, fmt.Errorf("stake expansion first input must be the previous staking transaction hash %s", m.PreviousStakingTxHash)
	}

	fundingTxHash := fundingTx.TxHash()
	if !stkExpandTx.TxIn[1].PreviousOutPoint.Hash.IsEqual(&fundingTxHash) {
		return nil, fmt.Errorf("stake expansion seccond input must be the given funding tx hash %s", fundingTxHash.String())
	}
	idxOtherInput := stkExpandTx.TxIn[1].PreviousOutPoint.Index

	if len(fundingTx.TxOut) > int(idxOtherInput) {
		return nil, fmt.Errorf("the given funding tx doesn't have the expected output %s", fundingTxHash.String())
	}
	otherOutput := fundingTx.TxOut[idxOtherInput]

	return &ParsedCreateDelStkExp{
		PreviousActiveStkTxHash: previousActiveStkTxHash,
		OtherFundingOutput:      otherOutput,
	}, nil
}

// ToParsed returns a parsed ParsedCreateDelegationMessage or error if it fails
func (m *MsgBtcStakeExpand) ToParsed() (*ParsedCreateDelegationMessage, error) {
	if m == nil {
		return nil, fmt.Errorf("cannot parse nil MsgCreateBTCDelegation")
	}

	return ParseCreateDelegationMessage(m)
}

// ValidateBasic does all the checks as MsgCreateBTCDelegation
// and verifies if the previous staking tx hash is valid
func (m *MsgBtcStakeExpand) ValidateBasic() error {
	_, err := chainhash.NewHashFromStr(m.PreviousStakingTxHash)
	if err != nil {
		return err
	}

	if _, err := ParseCreateDelegationMessage(m); err != nil {
		return err
	}

	return nil
}

func (m *MsgAddCovenantSigs) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Signer); err != nil {
		return fmt.Errorf("invalid signer addr: %s - %v", m.Signer, err)
	}
	if m.Pk == nil {
		return fmt.Errorf("empty BTC covenant public key")
	}
	if _, err := m.Pk.ToBTCPK(); err != nil {
		return fmt.Errorf("invalid BTC public key: %v", err)
	}
	if len(m.SlashingTxSigs) == 0 {
		return fmt.Errorf("empty covenant signatures on slashing tx")
	}
	if len(m.StakingTxHash) != chainhash.MaxHashStringSize {
		return fmt.Errorf("staking tx hash is not %d", chainhash.MaxHashStringSize)
	}

	// verifications about on-demand unbonding
	if m.UnbondingTxSig == nil {
		return fmt.Errorf("empty covenant signature")
	}

	if _, err := m.UnbondingTxSig.ToBTCSig(); err != nil {
		return fmt.Errorf("invalid covenant unbonding signature: %w", err)
	}

	if len(m.SlashingUnbondingTxSigs) == 0 {
		return fmt.Errorf("empty covenant signature")
	}

	return nil
}

func (m *MsgBTCUndelegate) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Signer); err != nil {
		return fmt.Errorf("invalid signer addr: %s - %v", m.Signer, err)
	}
	if len(m.StakingTxHash) != chainhash.MaxHashStringSize {
		return fmt.Errorf("staking tx hash is not %d", chainhash.MaxHashStringSize)
	}

	if m == nil {
		return fmt.Errorf("empty signature from the delegator")
	}

	if m.StakeSpendingTxInclusionProof == nil {
		return fmt.Errorf("empty inclusion proof")
	}

	if err := m.StakeSpendingTxInclusionProof.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid inclusion proof: %w", err)
	}

	if len(m.StakeSpendingTx) == 0 {
		return fmt.Errorf("empty delegator unbonding signature")
	}

	tx, err := bbn.NewBTCTxFromBytes(m.StakeSpendingTx)

	if err != nil {
		return fmt.Errorf("invalid stake spending tx tx: %w", err)
	}

	if err := blockchain.CheckTransactionSanity(btcutil.NewTx(tx)); err != nil {
		return fmt.Errorf("invalid stake spending tx: %w", err)
	}

	return nil
}

func (m *MsgAddBTCDelegationInclusionProof) ValidateBasic() error {
	if len(m.StakingTxHash) != chainhash.MaxHashStringSize {
		return fmt.Errorf("staking tx hash is not %d", chainhash.MaxHashStringSize)
	}

	if m.StakingTxInclusionProof == nil {
		return fmt.Errorf("empty inclusion proof")
	}

	if err := m.StakingTxInclusionProof.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid inclusion proof: %w", err)
	}

	if _, err := sdk.AccAddressFromBech32(m.Signer); err != nil {
		return fmt.Errorf("invalid signer addr: %s - %v", m.Signer, err)
	}

	return nil
}

func (m *MsgSelectiveSlashingEvidence) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Signer); err != nil {
		return fmt.Errorf("invalid signer addr: %s - %v", m.Signer, err)
	}
	if len(m.StakingTxHash) != chainhash.MaxHashStringSize {
		return fmt.Errorf("staking tx hash is not %d", chainhash.MaxHashStringSize)
	}

	if len(m.RecoveredFpBtcSk) != btcec.PrivKeyBytesLen {
		return fmt.Errorf("malformed BTC SK. Expected length: %d, got %d", btcec.PrivKeyBytesLen, len(m.RecoveredFpBtcSk))
	}

	return nil
}
