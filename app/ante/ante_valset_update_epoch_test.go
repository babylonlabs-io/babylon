package ante_test

import (
	"testing"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	bbnapp "github.com/babylonlabs-io/babylon/v2/app"
	"github.com/babylonlabs-io/babylon/v2/app/ante"
	appparams "github.com/babylonlabs-io/babylon/v2/app/params"
	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v2/testutil/mocks"
	epochingtypes "github.com/babylonlabs-io/babylon/v2/x/epoching/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	slashtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestBlockValsetUpdateAtEndOfEpoch(t *testing.T) {
	encCfg := bbnapp.GetEncodingConfig()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builderRandMsg := encCfg.TxConfig.NewTxBuilder()
	err := builderRandMsg.SetMsgs(
		banktypes.NewMsgSend(
			datagen.GenRandomAccount().GetAddress(),
			datagen.GenRandomAccount().GetAddress(),
			sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, 10)),
		),
	)
	require.NoError(t, err)
	randTx := builderRandMsg.GetTx()

	builderUnjail := encCfg.TxConfig.NewTxBuilder()
	err = builderUnjail.SetMsgs(
		slashtypes.NewMsgUnjail(datagen.GenRandomAccount().GetAddress().String()),
		randTx.GetMsgs()[0],
	)
	require.NoError(t, err)
	txWithUnjail := builderUnjail.GetTx()

	ek := mocks.NewMockEpochingKeeper(ctrl)
	ek.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{
		EpochNumber: 0, // epoch zero returns zero at GetLastBlockHeight
	}).AnyTimes()

	ctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger())
	ctxZeroEpoch := ctx.WithHeaderInfo(header.Info{
		Height: 0,
	})

	ctxAnotherHeight := ctx.WithHeaderInfo(header.Info{
		Height: 10,
	})

	tcs := []struct {
		name   string
		ante   ante.BlockValsetUpdateAtEndOfEpoch
		tx     signing.Tx
		ctx    sdk.Context
		expErr error
	}{
		{
			name:   "valid: unjail not at the end of epoch",
			ante:   ante.NewBlockValsetUpdateAtEndOfEpoch(ek),
			tx:     txWithUnjail,
			ctx:    ctxAnotherHeight,
			expErr: nil,
		},
		{
			name:   "valid: rand msg at the end of epoch",
			ante:   ante.NewBlockValsetUpdateAtEndOfEpoch(ek),
			tx:     randTx,
			ctx:    ctxZeroEpoch,
			expErr: nil,
		},
		{
			name:   "invalid: unjail at the end of epoch",
			ante:   ante.NewBlockValsetUpdateAtEndOfEpoch(ek),
			tx:     txWithUnjail,
			ctx:    ctxZeroEpoch,
			expErr: epochingtypes.ErrValsetUpdateAtEndBlock.Wrap("slashtypes.MsgUnjail is invalid at the end of epoch"),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			deco := tc.ante

			next := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
				return tc.ctx, nil
			}

			_, err := deco.AnteHandle(tc.ctx, tc.tx, false, next)
			if tc.expErr == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.EqualError(t, err, tc.expErr.Error())
		})
	}
}
