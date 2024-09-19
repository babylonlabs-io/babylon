package signetlaunch_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/app"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/signetlaunch"
	btcstktypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	"github.com/stretchr/testify/require"
)

func TestValidateSignatureSignedFPsFromData(t *testing.T) {
	bbnApp := app.NewTmpBabylonApp()
	cdc := bbnApp.AppCodec()
	// the chain ID in context needs to match the one used when creating the tx signature.
	chainID := "bbn-1"

	ctx := bbnApp.BaseApp.NewContextLegacy(true, tmproto.Header{Height: 1, ChainID: chainID, Time: time.Now().UTC()})
	buff := bytes.NewBufferString(v1.SignedFPsStr)
	simulateTx := false

	var d v1.DataSignedFps
	err := json.Unmarshal(buff.Bytes(), &d)
	require.NoError(t, err)

	antehandlerSigVerifier := buildAnteHandlerSigVerifier(t, bbnApp)

	fpAddrs := make(map[string]interface{}, len(d.SignedTxsFP))
	for _, txAny := range d.SignedTxsFP {
		txBytes, err := json.Marshal(txAny)
		require.NoError(t, err)

		// decodes the transaction
		tx, err := bbnApp.TxConfig().TxJSONDecoder()(txBytes)
		require.NoError(t, err)

		msgs := tx.GetMsgs()
		require.Len(t, msgs, 1)

		msg, ok := msgs[0].(*btcstktypes.MsgCreateFinalityProvider)
		require.True(t, ok)

		_, exist := fpAddrs[msg.Addr]
		require.False(t, exist)
		fpAddrs[msg.Addr] = nil

		require.NoError(t, msg.ValidateBasic())

		// loads messages from the tx, only one message per tx is allowed.
		msgsV2, err := tx.GetMsgsV2()
		require.NoError(t, err)
		require.Len(t, msgsV2, 1)

		msgV2 := msgsV2[0]
		signers, err := cdc.GetMsgV2Signers(msgV2)
		require.NoError(t, err)
		require.Len(t, signers, 1)

		// checks that the signer_infos corresponding address in the transaction
		// matches the FP address defined.
		signerAddrStr, err := cdc.InterfaceRegistry().SigningContext().AddressCodec().BytesToString(signers[0])
		require.NoError(t, err)

		signerBbnAddr, err := sdk.AccAddressFromBech32(signerAddrStr)
		require.NoError(t, err)

		require.Equal(t, msg.Addr, signerAddrStr)
		// Proof of Possession check only for type BIP340 as expected in the networks registry instructions
		require.NoError(t, msg.Pop.VerifyBIP340(signerBbnAddr, msg.BtcPk))

		// creates the account with the signer address and sets the
		// sequence and acc number to zero every time, for this reason
		// it needs to remove account right after, otherwise new accounts
		// would have account number +1 and the signature verification would fail.
		acc := bbnApp.AccountKeeper.NewAccountWithAddress(ctx, signerBbnAddr)
		require.NoError(t, acc.SetSequence(0))
		require.NoError(t, acc.SetAccountNumber(0))
		bbnApp.AccountKeeper.SetAccount(ctx, acc)

		_, err = antehandlerSigVerifier(ctx, tx, simulateTx)
		require.NoError(t, err)

		bbnApp.AccountKeeper.RemoveAccount(ctx, acc)
	}
}

func buildAnteHandlerSigVerifier(t *testing.T, bbnApp *app.BabylonApp) sdk.AnteHandler {
	cdc := bbnApp.AppCodec()

	txConfigOpts := authtx.ConfigOptions{
		TextualCoinMetadataQueryFn: txmodule.NewBankKeeperCoinMetadataQueryFn(bbnApp.GetBankKeeper()),
		EnabledSignModes:           []signing.SignMode{signing.SignMode_SIGN_MODE_DIRECT},
	}
	anteTxConfig, err := authtx.NewTxConfigWithOptions(
		codec.NewProtoCodec(cdc.InterfaceRegistry()),
		txConfigOpts,
	)
	require.NoError(t, err)

	svd := ante.NewSigVerificationDecorator(bbnApp.AppKeepers.AccountKeeper, anteTxConfig.SignModeHandler())
	spkd := ante.NewSetPubKeyDecorator(bbnApp.AppKeepers.AccountKeeper)
	return sdk.ChainAnteDecorators(spkd, svd)
}
