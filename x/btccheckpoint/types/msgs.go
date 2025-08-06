package types

import (
	"errors"
	fmt "fmt"
	"math"
	"math/big"

	txformat "github.com/babylonlabs-io/babylon/v3/btctxformatter"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	// Ensure that MsgInsertBTCSpvProof implements all functions of the Msg interface
	_ sdk.Msg = (*MsgInsertBTCSpvProof)(nil)
	_ sdk.Msg = (*MsgUpdateParams)(nil)
	// Ensure all msgs implement ValidateBasic
	_ sdk.HasValidateBasic = (*MsgUpdateParams)(nil)
	_ sdk.HasValidateBasic = (*MsgInsertBTCSpvProof)(nil)
)

// ParseTwoProofs Parse and Validate transactions which should contain OP_RETURN data.
// OP_RETURN bytes are not validated in any way. It is up to the caller attach
// semantic meaning and validity to those bytes.
// Returned ParsedProofs are in same order as raw proofs
func ParseTwoProofs(
	submitter sdk.AccAddress,
	proofs []*BTCSpvProof,
	powLimit *big.Int,
	expectedTag txformat.BabylonTag) (*RawCheckpointSubmission, error) {
	// Expecting as many proofs as many parts our checkpoint is composed of
	if len(proofs) != txformat.NumberOfParts {
		return nil, fmt.Errorf("expected at exactly valid op return transactions")
	}

	var parsedProofs []*ParsedProof

	for _, proof := range proofs {
		parsedProof, e :=
			ParseProof(
				proof.BtcTransaction,
				proof.BtcTransactionIndex,
				proof.MerkleNodes,
				proof.ConfirmingBtcHeader,
				powLimit,
			)

		if e != nil {
			return nil, e
		}

		parsedProofs = append(parsedProofs, parsedProof)
	}

	var checkpointData [][]byte

	for i, proof := range parsedProofs {
		if i > math.MaxUint8 || i < 0 {
			return nil, fmt.Errorf("expected at most 255 proofs but got %d", len(parsedProofs))
		}
		partIdxUint8 := uint8(i)
		data, err := txformat.GetCheckpointData(
			expectedTag,
			txformat.CurrentVersion,
			partIdxUint8,
			proof.OpReturnData,
		)

		if err != nil {
			return nil, err
		}
		checkpointData = append(checkpointData, data)
	}

	// at this point we know we have two correctly formatted babylon op return transactions
	// we need to check if parts match
	rawCkptData, err := txformat.ConnectParts(txformat.CurrentVersion, checkpointData[0], checkpointData[1])

	if err != nil {
		return nil, err
	}

	rawCheckpoint, err := txformat.DecodeRawCheckpoint(txformat.CurrentVersion, rawCkptData)

	if err != nil {
		return nil, err
	}

	sub := NewRawCheckpointSubmission(submitter, *parsedProofs[0], *parsedProofs[1], *rawCheckpoint)

	return &sub, nil
}

func ParseSubmission(
	m *MsgInsertBTCSpvProof,
	powLimit *big.Int,
	expectedTag txformat.BabylonTag) (*RawCheckpointSubmission, error) {
	if m == nil {
		return nil, errors.New("msgInsertBTCSpvProof can't nil")
	}

	address, err := sdk.AccAddressFromBech32(m.Submitter)

	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid submitter address: %s", err)
	}

	sub, err := ParseTwoProofs(address, m.Proofs, powLimit, expectedTag)

	if err != nil {
		return nil, err
	}

	return sub, nil
}

// ValidateBasic does a sanity check on the provided data.
func (m *MsgUpdateParams) ValidateBasic() error {
	if err := m.Params.Validate(); err != nil {
		return err
	}

	return nil
}

// ValidateBasic performs stateless checks.
func (m *MsgInsertBTCSpvProof) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Submitter); err != nil {
		return fmt.Errorf("invalid submitter address: %w", err)
	}

	if len(m.Proofs) == 0 {
		return errors.New("at least one proof must be provided")
	}

	for i, proof := range m.Proofs {
		if err := proof.Validate(); err != nil {
			return fmt.Errorf("proof[%d]: %w", i, err)
		}
	}
	return nil
}
