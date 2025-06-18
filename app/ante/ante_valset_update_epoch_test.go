package ante_test

import (
	"testing"

	"cosmossdk.io/core/header"
	bbnapp "github.com/babylonlabs-io/babylon/v2/app"
	"github.com/babylonlabs-io/babylon/v2/app/ante"
	appparams "github.com/babylonlabs-io/babylon/v2/app/params"
	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v2/testutil/mocks"
	epochingtypes "github.com/babylonlabs-io/babylon/v2/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestBlockValsetUpdateAtEndOfEpoch(t *testing.T) {
	encCfg := bbnapp.GetEncodingConfig()

	builder := encCfg.TxConfig.NewTxBuilder()
	err := builder.SetMsgs(
		banktypes.NewMsgSend(
			datagen.GenRandomAccount().GetAddress(),
			datagen.GenRandomAccount().GetAddress(),
			sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, 10)),
		),
	)
	require.NoError(t, err)

	ctx := sdk.Context{}.WithHeaderInfo(header.Info{
		Height: 0,
	})

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ek := mocks.NewMockEpochingKeeper(ctrl)
	// epoch zero returns zero at GetLastBlockHeight
	ek.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{
		EpochNumber: 0,
	}).AnyTimes()

	tcs := []struct {
		name   string
		ante   ante.BlockValsetUpdateAtEndOfEpoch
		tx     signing.Tx
		expErr bool
	}{
		{
			name:   "bad tx; unjail at the end of epoch",
			ante:   ante.NewBlockValsetUpdateAtEndOfEpoch(ek),
			expErr: true,
		},
	}

	for _, tc := range tcs {
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
