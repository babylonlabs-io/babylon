package types_test

import (
	"testing"

	errorsmod "cosmossdk.io/errors"

	bbntypes "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"

	"github.com/stretchr/testify/require"
)

func TestBTCSpvProof_Validate(t *testing.T) {
	validTx := []byte{0x01, 0x02}
	validMerkleNodes := make([]byte, 64) // 2 nodes (32 bytes each)
	validHeader := bbntypes.BTCHeaderBytes(make([]byte, 80))

	testCases := []struct {
		name   string
		proof  types.BTCSpvProof
		expErr string
	}{
		{
			name: "valid proof",
			proof: types.BTCSpvProof{
				BtcTransaction:      validTx,
				BtcTransactionIndex: 1,
				MerkleNodes:         validMerkleNodes,
				ConfirmingBtcHeader: &validHeader,
			},
		},
		{
			name: "empty btc_transaction",
			proof: types.BTCSpvProof{
				BtcTransaction:      nil,
				MerkleNodes:         validMerkleNodes,
				ConfirmingBtcHeader: &validHeader,
			},
			expErr: "btc_transaction must not be empty",
		},
		{
			name: "merkle_nodes length not divisible by 32",
			proof: types.BTCSpvProof{
				BtcTransaction:      validTx,
				MerkleNodes:         make([]byte, 33), // invalid length
				ConfirmingBtcHeader: &validHeader,
			},
			expErr: "merkle_nodes length must be divisible by 32",
		},
		{
			name: "nil confirming_btc_header",
			proof: types.BTCSpvProof{
				BtcTransaction:      validTx,
				MerkleNodes:         validMerkleNodes,
				ConfirmingBtcHeader: nil,
			},
			expErr: "confirming_btc_header must not be nil",
		},
		{
			name: "confirming_btc_header not 80 bytes",
			proof: types.BTCSpvProof{
				BtcTransaction: validTx,
				MerkleNodes:    validMerkleNodes,
				ConfirmingBtcHeader: func() *bbntypes.BTCHeaderBytes {
					h := bbntypes.BTCHeaderBytes(make([]byte, 79))
					return &h
				}(),
			},
			expErr: "confirming_btc_header must be exactly 80 bytes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.proof.Validate()
			if tc.expErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.True(t, errorsmod.IsOf(err, types.ErrInvalidBTCSpvProof))
			require.Contains(t, err.Error(), tc.expErr)
		})
	}
}
