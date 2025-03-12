package types

import (
	"errors"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	ed255192 "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
)

var (
	// Ensure that MsgInsertHeader implements all functions of the Msg interface
	_ sdk.Msg = (*MsgWrappedCreateValidator)(nil)
	_ sdk.Msg = (*MsgInjectedCheckpoint)(nil)
)

func NewMsgWrappedCreateValidator(msgCreateVal *stakingtypes.MsgCreateValidator, blsPK *bls12381.PublicKey, pop *ProofOfPossession) (*MsgWrappedCreateValidator, error) {
	return &MsgWrappedCreateValidator{
		Key: &BlsKey{
			Pubkey: blsPK,
			Pop:    pop,
		},
		MsgCreateValidator: msgCreateVal,
	}, nil
}

func (m *MsgWrappedCreateValidator) VerifyPoP(valPubkey cryptotypes.PubKey) bool {
	return m.Key.Pop.IsValid(*m.Key.Pubkey, valPubkey)
}

// ValidateBasic validates statelesss message elements
func (m *MsgWrappedCreateValidator) ValidateBasic() error {
	if m.MsgCreateValidator == nil {
		return errors.New("MsgCreateValidator is nil")
	}
	var pubKey ed255192.PubKey
	err := pubKey.Unmarshal(m.MsgCreateValidator.Pubkey.GetValue())
	if err != nil {
		return err
	}
	ok := m.VerifyPoP(&pubKey)
	if !ok {
		return errors.New("the proof-of-possession is not valid")
	}

	return nil
}

// UnpackInterfaces implements UnpackInterfacesMessage.UnpackInterfaces
// Needed since msg.MsgCreateValidator.Pubkey is in type Any
func (msg MsgWrappedCreateValidator) UnpackInterfaces(unpacker codectypes.AnyUnpacker) error {
	return msg.MsgCreateValidator.UnpackInterfaces(unpacker)
}
func (msg *MsgInjectedCheckpoint) ValidateBasic() error {
	if msg.Ckpt == nil {
		return errors.New("checkpoint is nil")
	}
	if msg.ExtendedCommitInfo == nil {
		return errors.New("checkpoint is nil")
	}
	return nil
}

// GetSigners returns an empty slice as this is an internal message
func (msg *MsgInjectedCheckpoint) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{}
}

// Type returns the message type
func (msg *MsgInjectedCheckpoint) Type() string {
	return "injected_checkpoint"
}

// Route returns the module name
func (msg *MsgInjectedCheckpoint) Route() string {
	return ModuleName
}

// GetSignBytes returns nil as this is an internal message
func (msg *MsgInjectedCheckpoint) GetSignBytes() []byte {
	return nil
}

// UnpackInterfaces implements UnpackInterfacesMessage.UnpackInterfaces
func (msg MsgInjectedCheckpoint) UnpackInterfaces(unpacker codectypes.AnyUnpacker) error {
	// No interfaces to unpack for this message
	return nil
}
