package types

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
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

	if m == nil {
		return fmt.Errorf("empty signature from the delegator")
	}

	if m.StakeSpendingTxInclusionProof == nil {
		return fmt.Errorf("empty inclusion proof")
	}

	if err := m.StakeSpendingTxInclusionProof.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid inclusion proof: %w", err)
	}

	if m.StakeSpendingTx == nil {
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
