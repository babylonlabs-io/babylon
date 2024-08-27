package types

import (
	"fmt"

	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ensure that these message types implement the sdk.Msg interface
var (
	_ sdk.Msg = &MsgUpdateParams{}
	_ sdk.Msg = &MsgCreateFinalityProvider{}
	_ sdk.Msg = &MsgEditFinalityProvider{}
	_ sdk.Msg = &MsgCreateBTCDelegation{}
	_ sdk.Msg = &MsgAddCovenantSigs{}
	_ sdk.Msg = &MsgBTCUndelegate{}
)

func (m *MsgCreateFinalityProvider) ValidateBasic() error {
	if m.Commission == nil {
		return fmt.Errorf("empty commission")
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
	if m.Commission == nil {
		return fmt.Errorf("empty commission")
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

func (m *MsgCreateBTCDelegation) ValidateBasic() error {
	if _, err := ParseCreateDelegationMessage(m); err != nil {
		return err
	}

	return nil
}

func (m *MsgAddCovenantSigs) ValidateBasic() error {
	if m.Pk == nil {
		return fmt.Errorf("empty BTC covenant public key")
	}
	if _, err := m.Pk.ToBTCPK(); err != nil {
		return fmt.Errorf("invalid BTC public key: %v", err)
	}
	if m.SlashingTxSigs == nil {
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

	if m.SlashingUnbondingTxSigs == nil {
		return fmt.Errorf("empty covenant signature")
	}

	return nil
}

func (m *MsgBTCUndelegate) ValidateBasic() error {
	if len(m.StakingTxHash) != chainhash.MaxHashStringSize {
		return fmt.Errorf("staking tx hash is not %d", chainhash.MaxHashStringSize)
	}

	if m.UnbondingTxSig == nil {
		return fmt.Errorf("empty signature from the delegator")
	}

	if _, err := m.UnbondingTxSig.ToBTCSig(); err != nil {
		return fmt.Errorf("invalid delegator unbonding signature: %w", err)
	}

	return nil
}
