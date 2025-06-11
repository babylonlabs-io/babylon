package types

import (
	"errors"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	ed255192 "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/crypto/bls12381"
)

var (
	// Ensure that MsgInsertHeader implements all functions of the Msg interface
	_ sdk.Msg = (*MsgWrappedCreateValidator)(nil)
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

	if m.Key == nil {
		return errors.New("BLS key is nil")
	}

	cdc := authcodec.NewBech32Codec(appparams.Bech32PrefixValAddr)
	if err := m.MsgCreateValidator.Validate(cdc); err != nil {
		return err
	}

	if err := m.Key.ValidateBasic(); err != nil {
		return err
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
