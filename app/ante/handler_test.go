package ante_test

import (
	"fmt"
	"testing"

	log "cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	"cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/babylonlabs-io/babylon/app/ante"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	ckpttypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	db "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TestTx struct {
	msgs          []sdk.Msg
	timeoutHeight uint64
}

func (tx TestTx) GetMsgs() []sdk.Msg { return tx.msgs }
func (tx TestTx) GetMsgsV2() ([]protoreflect.ProtoMessage, error) {
	var protoMsgs []protoreflect.ProtoMessage
	for _, msg := range tx.msgs {
		if protoMsg, ok := msg.(protoreflect.ProtoMessage); ok {
			protoMsgs = append(protoMsgs, protoMsg)
		} else {
			return nil, fmt.Errorf("message does not implement protoreflect.ProtoMessage")
		}
	}
	return protoMsgs, nil
}

func (tx TestTx) ValidateBasic() error       { return nil }
func (tx TestTx) GetGas() uint64             { return 0 }
func (tx TestTx) GetFee() sdk.Coins          { return sdk.NewCoins() }
func (tx TestTx) FeePayer() sdk.AccAddress   { return nil }
func (tx TestTx) FeeGranter() sdk.AccAddress { return nil }
func (tx TestTx) GetMemo() string            { return "" }
func (tx TestTx) GetTimeoutHeight() uint64 {
	return tx.timeoutHeight
}

func setupTestContext(t *testing.T) (sdk.Context, ante.AppAnteHandler) {
	storekey := types.NewKVStoreKey("test")
	logger := log.NewTestLogger(t)
	memDB := db.NewMemDB()

	ms := store.NewCommitMultiStore(memDB, logger, storemetrics.NewNoOpMetrics())
	ms.MountStoreWithDB(storekey, types.StoreTypeIAVL, memDB)
	require.NoError(t, ms.LoadLatestVersion())

	ctx := sdk.NewContext(ms, cmtproto.Header{}, false, log.NewNopLogger())
	accountKeeper := keepertest.AccountKeeper(t, memDB, ms)
	handler := ante.NewAppAnteHandler(accountKeeper)
	return ctx, handler
}
func TestAppAnteHandler(t *testing.T) {
	ctx, handler := setupTestContext(t)

	tests := []struct {
		name      string
		setup     func() (sdk.Context, TestTx)
		expectErr bool
	}{
		{
			name: "internal message passes in deliver mode",
			setup: func() (sdk.Context, TestTx) {
				tx := TestTx{
					msgs: []sdk.Msg{
						&ckpttypes.MsgInjectedCheckpoint{},
					},
				}
				return ctx, tx
			},
			expectErr: false,
		},
		{
			name: "no messages",
			setup: func() (sdk.Context, TestTx) {
				tx := TestTx{
					msgs: []sdk.Msg{},
				}
				return ctx, tx
			},
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, tx := tc.setup()
			_, err := handler.AppInjectedMsgAnteHandle(ctx, tx, false)

			if tc.expectErr {
				require.Error(t, err, "expected an error but got none")
			} else {
				require.NoError(t, err, "unexpected error: %v", err)
			}
		})
	}
}

