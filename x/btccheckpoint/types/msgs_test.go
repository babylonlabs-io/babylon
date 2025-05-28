package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btccheckpoint/types"

	"github.com/stretchr/testify/require"
)

func TestMsgInsertBTCSpvProof_ValidateBasic(t *testing.T) {
	validAddr := datagen.GenRandomAddress().String()
	validTx := []byte{0x01, 0x02}
	validMerkleNodes := make([]byte, 64) // 2 nodes
	validHeader := bbntypes.BTCHeaderBytes(make([]byte, 80))

	validProof := &types.BTCSpvProof{
		BtcTransaction:      validTx,
		BtcTransactionIndex: 1,
		MerkleNodes:         validMerkleNodes,
		ConfirmingBtcHeader: &validHeader,
	}

	invalidProof := &types.BTCSpvProof{
		BtcTransaction:      nil,
		MerkleNodes:         validMerkleNodes,
		ConfirmingBtcHeader: &validHeader,
	}

	testCases := []struct {
		name   string
		msg    types.MsgInsertBTCSpvProof
		expErr string
	}{
		{
			name: "valid msg",
			msg: types.MsgInsertBTCSpvProof{
				Submitter: validAddr,
				Proofs:    []*types.BTCSpvProof{validProof},
			},
		},
		{
			name: "invalid submitter address",
			msg: types.MsgInsertBTCSpvProof{
				Submitter: "not-a-valid-address",
				Proofs:    []*types.BTCSpvProof{validProof},
			},
			expErr: "invalid submitter address",
		},
		{
			name: "empty proofs",
			msg: types.MsgInsertBTCSpvProof{
				Submitter: validAddr,
				Proofs:    []*types.BTCSpvProof{},
			},
			expErr: "at least one proof must be provided",
		},
		{
			name: "invalid proof inside list",
			msg: types.MsgInsertBTCSpvProof{
				Submitter: validAddr,
				Proofs:    []*types.BTCSpvProof{invalidProof},
			},
			expErr: "proof[0]:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expErr)
		})
	}
}
