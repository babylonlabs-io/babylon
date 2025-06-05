package ante_test

import (
	"testing"

	sdktestdata "github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v2/app/ante"
)

func TestIBCMsgSizeDecorator(t *testing.T) {
	var (
		validMemo       = "hello"
		longMemo        = string(make([]byte, ante.MaxMemoSize+1))
		validReceiver   = "cosmos1validaddress"
		longReceiver    = string(make([]byte, ante.MaxAddressSize+1))
		oversizedPacket = icatypes.InterchainAccountPacketData{
			Type: icatypes.EXECUTE_TX,
			Data: make([]byte, ante.MaxMsgSize+1),
			Memo: validMemo,
		}
		validPacket = icatypes.InterchainAccountPacketData{
			Type: icatypes.EXECUTE_TX,
			Data: make([]byte, ante.MaxMsgSize/2),
		}
	)

	tests := []struct {
		name      string
		msgs      []sdk.Msg
		isCheckTx bool
		errMsg    string
	}{
		{
			name:      "Empty msgs",
			msgs:      []sdk.Msg{},
			isCheckTx: true,
		},
		{
			name: "Valid MsgTransfer",
			msgs: []sdk.Msg{
				&ibctransfertypes.MsgTransfer{
					Memo:     validMemo,
					Receiver: validReceiver,
				},
			},
			isCheckTx: true,
		},
		{
			name: "MsgTransfer exceeds size",
			msgs: []sdk.Msg{
				&ibctransfertypes.MsgTransfer{
					Memo:     longMemo + longMemo,
					Receiver: longReceiver,
				},
			},
			isCheckTx: true,
			errMsg:    "msg size is too large",
		},
		{
			name: "MsgTransfer memo too long",
			msgs: []sdk.Msg{
				&ibctransfertypes.MsgTransfer{
					Memo:     longMemo,
					Receiver: validReceiver,
				},
			},
			isCheckTx: true,
			errMsg:    "memo is too large",
		},
		{
			name: "MsgTransfer memo too long without check tx",
			msgs: []sdk.Msg{
				&ibctransfertypes.MsgTransfer{
					Memo:     longMemo,
					Receiver: validReceiver,
				},
			},
			isCheckTx: false,
			errMsg:    "memo is too large",
		},
		{
			name: "MsgTransfer receiver address too long",
			msgs: []sdk.Msg{
				&ibctransfertypes.MsgTransfer{
					Memo:     validMemo,
					Receiver: longReceiver,
				},
			},
			isCheckTx: true,
			errMsg:    "receiver address is too large",
		},
		{
			name: "Valid MsgSendTx",
			msgs: []sdk.Msg{
				&icacontrollertypes.MsgSendTx{
					Owner:      validReceiver,
					PacketData: validPacket,
				},
			},
			isCheckTx: true,
		},
		{
			name: "MsgSendTx packet data too large",
			msgs: []sdk.Msg{
				&icacontrollertypes.MsgSendTx{
					Owner:      validReceiver,
					PacketData: oversizedPacket,
				},
			},
			isCheckTx: true,
			errMsg:    "packet data is too large",
		},
		{
			name: "MsgSendTx owner address too large",
			msgs: []sdk.Msg{
				&icacontrollertypes.MsgSendTx{
					Owner:      longReceiver,
					PacketData: validPacket,
				},
			},
			isCheckTx: true,
			errMsg:    "owner address is too large",
		},
		{
			name: "Non-IBC message (should pass)",
			msgs: []sdk.Msg{
				&sdktestdata.TestMsg{},
			},
			isCheckTx: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := sdk.Context{}.WithIsCheckTx(tc.isCheckTx)
			tx := sdk.Tx(mockTx{msgs: tc.msgs})
			deco := ante.NewIBCMsgSizeDecorator()

			next := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
				return ctx, nil
			}

			_, err := deco.AnteHandle(ctx, tx, false, next)
			if tc.errMsg == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.errMsg)
		})
	}
}
