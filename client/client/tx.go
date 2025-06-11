package client

import (
	"context"
	"fmt"
	"sync"

	signingv1beta1 "cosmossdk.io/api/cosmos/tx/signing/v1beta1"
	"cosmossdk.io/errors"
	txsigning "cosmossdk.io/x/tx/signing"
	"github.com/avast/retry-go/v4"
	"github.com/babylonlabs-io/babylon/v3/client/babylonclient"
	btcctypes "github.com/babylonlabs-io/babylon/v3/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	abci "github.com/cometbft/cometbft/abci/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"go.uber.org/zap"
)

// ToProviderMsgs converts a list of sdk.Msg to a list of provider.RelayerMessage
func ToProviderMsgs(msgs []sdk.Msg) []babylonclient.RelayerMessage {
	relayerMsgs := make([]babylonclient.RelayerMessage, 0, len(msgs))
	for _, m := range msgs {
		relayerMsgs = append(relayerMsgs, babylonclient.NewCosmosMessage(m, func(signer string) {}))
	}
	return relayerMsgs
}

// SendMsgToMempool sends a message to the mempool.
// It does not wait for the messages to be included.
func (c *Client) SendMsgToMempool(ctx context.Context, msg sdk.Msg) error {
	return c.SendMsgsToMempool(ctx, []sdk.Msg{msg})
}

// SendMsgsToMempool sends a set of messages to the mempool.
// It does not wait for the messages to be included.
func (c *Client) SendMsgsToMempool(ctx context.Context, msgs []sdk.Msg) error {
	relayerMsgs := ToProviderMsgs(msgs)
	if err := retry.Do(func() error {
		var sendMsgErr error
		krErr := c.accessKeyWithLock(func() {
			sendMsgErr = c.provider.SendMessagesToMempool(ctx, relayerMsgs, "", ctx, []func(*babylonclient.RelayerTxResponse, error){})
		})
		if krErr != nil {
			c.logger.Error("unrecoverable err when submitting the tx, skip retrying", zap.Error(krErr))
			return retry.Unrecoverable(krErr)
		}
		return sendMsgErr
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		c.logger.Debug("retrying", zap.Uint("attempt", n+1), zap.Uint("max_attempts", rtyAttNum), zap.Error(err))
	})); err != nil {
		return err
	}

	return nil
}

// SendMsg sends a message to the chain.
func (c *Client) SendMsg(ctx context.Context, msg sdk.Msg, expectedErrors []*errors.Error, unrecoverableErrors []*errors.Error) (*babylonclient.RelayerTxResponse, error) {
	return c.SendMsgs(ctx, []sdk.Msg{msg}, expectedErrors, unrecoverableErrors)
}

// SendMsgs sends a list of messages to the chain.
func (c *Client) SendMsgs(ctx context.Context, msgs []sdk.Msg, expectedErrors []*errors.Error, unrecoverableErrors []*errors.Error) (*babylonclient.RelayerTxResponse, error) {
	return c.ReliablySendMsgs(ctx, msgs, expectedErrors, unrecoverableErrors, 1)
}

// ReliablySendMsg reliable sends a message to the chain.
// It utilizes a file lock as well as a keyring lock to ensure atomic access.
// TODO: needs tests
func (c *Client) ReliablySendMsg(ctx context.Context, msg sdk.Msg, expectedErrors []*errors.Error, unrecoverableErrors []*errors.Error, retries ...uint) (*babylonclient.RelayerTxResponse, error) {
	return c.ReliablySendMsgs(ctx, []sdk.Msg{msg}, expectedErrors, unrecoverableErrors, retries...)
}

// ReliablySendMsgs reliably sends a list of messages to the chain.
// It utilizes a file lock as well as a keyring lock to ensure atomic access.
// TODO: needs tests
func (c *Client) ReliablySendMsgs(ctx context.Context, msgs []sdk.Msg, expectedErrors []*errors.Error, unrecoverableErrors []*errors.Error, retries ...uint) (*babylonclient.RelayerTxResponse, error) {
	var (
		rlyResp     *babylonclient.RelayerTxResponse
		callbackErr error
		wg          sync.WaitGroup
	)

	rty := rtyAttNum
	rtyAttempts := rtyAtt
	if len(retries) > 0 {
		rty = retries[0]
		rtyAttempts = retry.Attempts(rty)
	}

	callback := func(rtr *babylonclient.RelayerTxResponse, err error) {
		rlyResp = rtr
		callbackErr = err
		wg.Done()
	}

	wg.Add(1)

	// convert message type
	relayerMsgs := ToProviderMsgs(msgs)

	// TODO: consider using Babylon's retry package
	if err := retry.Do(func() error {
		var sendMsgErr error
		krErr := c.accessKeyWithLock(func() {
			sendMsgErr = c.provider.SendMessagesToMempool(ctx, relayerMsgs, "", ctx, []func(*babylonclient.RelayerTxResponse, error){callback})
		})
		if krErr != nil {
			c.logger.Error("unrecoverable err when submitting the tx, skip retrying", zap.Error(krErr))
			return retry.Unrecoverable(krErr)
		}
		if sendMsgErr != nil {
			if errorContained(sendMsgErr, unrecoverableErrors) {
				c.logger.Error("unrecoverable err when submitting the tx, skip retrying", zap.Error(sendMsgErr))
				return retry.Unrecoverable(sendMsgErr)
			}
			if errorContained(sendMsgErr, expectedErrors) {
				// this is necessary because if err is returned
				// the callback function will not be executed so
				// that the inside wg.Done will not be executed
				wg.Done()
				c.logger.Error("expected err when submitting the tx, skip retrying", zap.Error(sendMsgErr))
				return nil
			}
			return sendMsgErr
		}
		return nil
	}, retry.Context(ctx), rtyAttempts, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		c.logger.Debug("retrying", zap.Uint("attempt", n+1), zap.Uint("max_attempts", rty), zap.Error(err))
	})); err != nil {
		return nil, err
	}

	wg.Wait()

	if callbackErr != nil {
		if errorContained(callbackErr, expectedErrors) {
			return nil, nil
		}
		return nil, callbackErr
	}

	if rlyResp == nil {
		// this case could happen if the error within the retry is an expected error
		return nil, nil
	}

	if rlyResp.Code != 0 {
		return rlyResp, fmt.Errorf("transaction failed with code: %d", rlyResp.Code)
	}

	return rlyResp, nil
}

// ReliablySendMsgsWithSigner reliably sends a list of messages to the chain.
// It utilizes the signer private key to sign all msgs
func (c *Client) ReliablySendMsgsWithSigner(ctx context.Context, signerAddr sdk.AccAddress, signerPvKey *secp256k1.PrivKey, msgs []sdk.Msg, expectedErrors []*errors.Error, unrecoverableErrors []*errors.Error) (*babylonclient.RelayerTxResponse, error) {
	var (
		rlyResp     *babylonclient.RelayerTxResponse
		callbackErr error
		wg          sync.WaitGroup
	)
	wg.Add(1)

	// convert message type
	relayerMsgs := ToProviderMsgs(msgs)

	// TODO: consider using Babylon's retry package
	if err := retry.Do(func() error {
		_, sendMsgErr := c.SendMessageWithSigner(ctx, signerAddr, signerPvKey, relayerMsgs)
		if sendMsgErr != nil {
			if errorContained(sendMsgErr, unrecoverableErrors) {
				c.logger.Error("unrecoverable err when submitting the tx, skip retrying", zap.Error(sendMsgErr))
				return retry.Unrecoverable(sendMsgErr)
			}
			if errorContained(sendMsgErr, expectedErrors) {
				// this is necessary because if err is returned
				// the callback function will not be executed so
				// that the inside wg.Done will not be executed
				wg.Done()
				c.logger.Error("expected err when submitting the tx, skip retrying", zap.Error(sendMsgErr))
				return nil
			}
			return sendMsgErr
		}
		wg.Done()
		return nil
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		c.logger.Debug("retrying", zap.Uint("attempt", n+1), zap.Uint("max_attempts", rtyAttNum), zap.Error(err))
	})); err != nil {
		return nil, err
	}

	wg.Wait()

	if callbackErr != nil {
		if errorContained(callbackErr, expectedErrors) {
			return nil, nil
		}
		return nil, callbackErr
	}

	if rlyResp == nil {
		// this case could happen if the error within the retry is an expected error
		return nil, nil
	}

	if rlyResp.Code != 0 {
		return rlyResp, fmt.Errorf("transaction failed with code: %d", rlyResp.Code)
	}

	return rlyResp, nil
}

func (c *Client) SendMessageWithSigner(
	ctx context.Context,
	signerAddr sdk.AccAddress,
	signerPvKey *secp256k1.PrivKey,
	relayerMsgs []babylonclient.RelayerMessage,
) (result *coretypes.ResultBroadcastTx, err error) {
	cMsgs := babylonclient.CosmosMsgs(relayerMsgs...)
	var (
		num, seq uint64
	)

	cc := c.provider
	cliCtx := client.Context{}.WithClient(cc.RPCClient).
		WithInterfaceRegistry(cc.Cdc.InterfaceRegistry).
		WithChainID(cc.PCfg.ChainID).
		WithCodec(cc.Cdc.Codec).
		WithFromAddress(signerAddr)

	txf := cc.NewTxFactory()
	if err := retry.Do(func() error {
		if err := txf.AccountRetriever().EnsureExists(cliCtx, signerAddr); err != nil {
			return err
		}
		return err
	}, rtyAtt, rtyDel, rtyErr); err != nil {
		return nil, err
	}

	initNum, initSeq := txf.AccountNumber(), txf.Sequence()
	if initNum == 0 || initSeq == 0 {
		if err := retry.Do(func() error {
			num, seq, err = txf.AccountRetriever().GetAccountNumberSequence(cliCtx, signerAddr)
			if err != nil {
				return err
			}
			return err
		}, rtyAtt, rtyDel, rtyErr); err != nil {
			return nil, err
		}

		if initNum == 0 {
			txf = txf.WithAccountNumber(num)
		}

		if initSeq == 0 {
			txf = txf.WithSequence(seq)
		}
	}

	if cc.PCfg.MinGasAmount != 0 {
		txf = txf.WithGas(cc.PCfg.MinGasAmount)
	}

	if cc.PCfg.MaxGasAmount != 0 {
		txf = txf.WithGas(cc.PCfg.MaxGasAmount)
	}

	// txf ready
	_, adjusted, err := c.CalculateGas(ctx, txf, signerPvKey.PubKey(), cMsgs...)
	if err != nil {
		return nil, err
	}

	// Set the gas amount on the transaction factory
	txf = txf.WithGas(adjusted)

	// Build the transaction builder
	txb, err := txf.BuildUnsignedTx(cMsgs...)
	if err != nil {
		return nil, err
	}

	// Attach the signature to the transaction
	// c.LogFailedTx(nil, err, msgs)
	// Force encoding in the chain specific address
	for _, msg := range cMsgs {
		cc.Cdc.Codec.MustMarshalJSON(msg)
	}
	if err := Sign(ctx, txf, signerPvKey, txb, cc.Cdc.TxConfig.SignModeHandler(), false); err != nil {
		return nil, err
	}

	tx := txb.GetTx()

	// Generate the transaction bytes
	txBytes, err := cc.Cdc.TxConfig.TxEncoder()(tx)
	if err != nil {
		return nil, err
	}

	return cc.RPCClient.BroadcastTxSync(ctx, txBytes)
}

// BuildSimTx creates an unsigned tx with an empty single signature and returns
// the encoded transaction or an error if the unsigned transaction cannot be built.
func BuildSimTx(pk cryptotypes.PubKey, txf tx.Factory, msgs ...sdk.Msg) ([]byte, error) {
	txb, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, err
	}

	// Create an empty signature literal as the ante handler will populate with a
	// sentinel pubkey.
	sig := signing.SignatureV2{
		PubKey: pk,
		Data: &signing.SingleSignatureData{
			SignMode: txf.SignMode(),
		},
		Sequence: txf.Sequence(),
	}
	if err := txb.SetSignatures(sig); err != nil {
		return nil, err
	}

	protoProvider, ok := txb.(protoTxProvider)
	if !ok {
		return nil, fmt.Errorf("cannot simulate amino tx")
	}

	simReq := txtypes.SimulateRequest{Tx: protoProvider.GetProtoTx()}
	return simReq.Marshal()
}

// CalculateGas simulates a tx to generate the appropriate gas settings before broadcasting a tx.
func (c *Client) CalculateGas(ctx context.Context, txf tx.Factory, signingPK cryptotypes.PubKey, msgs ...sdk.Msg) (txtypes.SimulateResponse, uint64, error) {
	cc := c.provider

	var txBytes []byte
	if err := retry.Do(func() error {
		var err error
		txBytes, err = BuildSimTx(signingPK, txf, msgs...)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr); err != nil {
		return txtypes.SimulateResponse{}, 0, err
	}

	simQuery := abci.RequestQuery{
		Path: "/cosmos.tx.v1beta1.Service/Simulate",
		Data: txBytes,
	}

	var res abci.ResponseQuery
	if err := retry.Do(func() error {
		var err error
		res, err = cc.QueryABCI(ctx, simQuery)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr); err != nil {
		return txtypes.SimulateResponse{}, 0, err
	}

	var simRes txtypes.SimulateResponse
	if err := simRes.Unmarshal(res.Value); err != nil {
		return txtypes.SimulateResponse{}, 0, err
	}

	gas, err := cc.AdjustEstimatedGas(simRes.GasInfo.GasUsed)
	return simRes, gas, err
}

// Sign signs a given tx with the private key. The bytes signed over are canconical.
// The resulting signature will be added to the transaction builder overwriting the previous
// ones if overwrite=true (otherwise, the signature will be appended).
// Signing a transaction with mutltiple signers in the DIRECT mode is not supported and will
// return an error.
// An error is returned upon failure.
func Sign(
	ctx context.Context,
	txf tx.Factory,
	signerPvKey *secp256k1.PrivKey,
	txBuilder client.TxBuilder,
	handlerMap *txsigning.HandlerMap,
	overwriteSig bool,
) error {
	var err error
	signMode := txf.SignMode()
	if signMode == signing.SignMode_SIGN_MODE_UNSPECIFIED {
		// use the SignModeHandler's default mode if unspecified
		signMode, err = authsigning.APISignModeToInternal(signingv1beta1.SignMode_SIGN_MODE_DIRECT_AUX)
		if err != nil {
			return err
		}
	}

	pubKey := signerPvKey.PubKey()

	signerData := authsigning.SignerData{
		ChainID:       txf.ChainID(),
		AccountNumber: txf.AccountNumber(),
		Sequence:      txf.Sequence(),
		PubKey:        pubKey,
		Address:       sdk.AccAddress(pubKey.Address()).String(),
	}

	// For SIGN_MODE_DIRECT, calling SetSignatures calls setSignerInfos on
	// TxBuilder under the hood, and SignerInfos is needed to generated the
	// sign bytes. This is the reason for setting SetSignatures here, with a
	// nil signature.
	//
	// Note: this line is not needed for SIGN_MODE_LEGACY_AMINO, but putting it
	// also doesn't affect its generated sign bytes, so for code's simplicity
	// sake, we put it here.
	sigData := signing.SingleSignatureData{
		SignMode:  signMode,
		Signature: nil,
	}
	sig := signing.SignatureV2{
		PubKey:   pubKey,
		Data:     &sigData,
		Sequence: txf.Sequence(),
	}

	var prevSignatures []signing.SignatureV2
	if !overwriteSig {
		prevSignatures, err = txBuilder.GetTx().GetSignaturesV2()
		if err != nil {
			return err
		}
	}
	// Overwrite or append signer infos.
	var sigs []signing.SignatureV2
	if overwriteSig {
		sigs = []signing.SignatureV2{sig}
	} else {
		sigs = append(sigs, prevSignatures...)
		sigs = append(sigs, sig)
	}
	if err := txBuilder.SetSignatures(sigs...); err != nil {
		return err
	}

	bytesToSign, err := authsigning.GetSignBytesAdapter(ctx, handlerMap, signMode, signerData, txBuilder.GetTx())
	if err != nil {
		return err
	}

	sigBytes, err := signerPvKey.Sign(bytesToSign)
	if err != nil {
		return err
	}

	// Construct the SignatureV2 struct
	sigData = signing.SingleSignatureData{
		SignMode:  signMode,
		Signature: sigBytes,
	}
	sig = signing.SignatureV2{
		PubKey:   pubKey,
		Data:     &sigData,
		Sequence: txf.Sequence(),
	}

	if overwriteSig {
		err = txBuilder.SetSignatures(sig)
	} else {
		prevSignatures = append(prevSignatures, sig)
		err = txBuilder.SetSignatures(prevSignatures...)
	}

	if err != nil {
		return fmt.Errorf("unable to set signatures on payload: %w", err)
	}

	// Run optional preprocessing if specified. By default, this is unset
	// and will return nil.
	return nil
}

// We do not expose ctx in our client calls, which means:
// - we do not support cancellation of submitting messages
// - the only timeout is the block inclusion timeout i.e block-timeout
// TODO: To properly support cancellation we need to expose ctx in our client calls
func (c *Client) InsertBTCSpvProof(ctx context.Context, msg *btcctypes.MsgInsertBTCSpvProof) (*babylonclient.RelayerTxResponse, error) {
	return c.ReliablySendMsg(ctx, msg, []*errors.Error{}, []*errors.Error{})
}

func (c *Client) InsertHeaders(ctx context.Context, msg *btclctypes.MsgInsertHeaders) (*babylonclient.RelayerTxResponse, error) {
	return c.ReliablySendMsg(ctx, msg, []*errors.Error{}, []*errors.Error{})
}

// protoTxProvider is a type which can provide a proto transaction. It is a
// workaround to get access to the wrapper TxBuilder's method GetProtoTx().
type protoTxProvider interface {
	GetProtoTx() *txtypes.Tx
}

// TODO: implement necessary message invocations here
// - MsgInconsistencyEvidence
// - MsgStallingEvidence
