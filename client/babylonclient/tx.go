// This file is derived from the Cosmos Relayer repository (https://github.com/cosmos/relayer),
// originally licensed under the Apache License, Version 2.0.

package babylonclient

import (
	"context"
	sdkerrors "cosmossdk.io/errors"
	"cosmossdk.io/store/rootmulti"
	"errors"
	"fmt"
	abci "github.com/cometbft/cometbft/abci/types"
	client2 "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	legacyerrors "github.com/cosmos/cosmos-sdk/types/errors"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
)

var (
	rtyAttNum                   = uint(5)
	rtyAtt                      = retry.Attempts(rtyAttNum)
	rtyDel                      = retry.Delay(time.Millisecond * 400)
	rtyErr                      = retry.LastErrorOnly(true)
	accountSeqRegex             = regexp.MustCompile("account sequence mismatch, expected ([0-9]+), got ([0-9]+)")
	defaultBroadcastWaitTimeout = 10 * time.Minute
	errUnknown                  = "unknown"
)

const (
	ErrTimeoutAfterWaitingForTxBroadcast _err = "timed out after waiting for tx to get included in the block"
)

type _err string

func (e _err) Error() string { return string(e) }

type intoAny interface {
	AsAny() *codectypes.Any
}

var seqGuardSingleton sync.Mutex

// Gets the sequence guard. If it doesn't exist, initialized and returns it.
func ensureSequenceGuard(cc *CosmosProvider, key string) *WalletState {
	seqGuardSingleton.Lock()
	defer seqGuardSingleton.Unlock()

	if cc.walletStateMap == nil {
		cc.walletStateMap = map[string]*WalletState{}
	}

	sequenceGuard, ok := cc.walletStateMap[key]
	if !ok {
		cc.walletStateMap[key] = &WalletState{}
		return cc.walletStateMap[key]
	}

	return sequenceGuard
}

// QueryABCI performs an ABCI query and returns the appropriate response and error sdk error code.
func (cc *CosmosProvider) QueryABCI(ctx context.Context, req abci.RequestQuery) (abci.ResponseQuery, error) {
	opts := client2.ABCIQueryOptions{
		Height: req.Height,
		Prove:  req.Prove,
	}

	result, err := cc.RPCClient.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)
	if err != nil {
		return abci.ResponseQuery{}, err
	}

	if !result.Response.IsOK() {
		return abci.ResponseQuery{}, sdkErrorToGRPCError(result.Response)
	}

	// data from trusted node or subspace query doesn't need verification
	if !opts.Prove || !isQueryStoreWithProof(req.Path) {
		return result.Response, nil
	}

	return result.Response, nil
}

// broadcastTx broadcasts a transaction with the given raw bytes and then, in an async goroutine, waits for the tx to be included in the block.
// The wait will end after either the asyncTimeout has run out or the asyncCtx exits.
// If there is no error broadcasting, the asyncCallback will be called with success/failure of the wait for block inclusion.
func (cc *CosmosProvider) broadcastTx(
	ctx context.Context, // context for tx broadcast
	tx []byte, // raw tx to be broadcast
	asyncCtx context.Context, // context for async wait for block inclusion after successful tx broadcast
	asyncTimeout time.Duration, // timeout for waiting for block inclusion
	asyncCallbacks []func(*RelayerTxResponse, error), // callback for success/fail of the wait for block inclusion
) error {
	res, err := cc.RPCClient.BroadcastTxSync(ctx, tx)
	isErr := err != nil
	isFailed := res != nil && res.Code != 0
	if isErr || isFailed {
		if isErr && res == nil {
			// There are some cases where BroadcastTxSync will return an error but the associated
			// ResultBroadcastTx will be nil.
			return err
		}
		if isFailed {
			if err = cc.sdkError(res.Codespace, res.Code); err == nil {
				err = fmt.Errorf("transaction failed to execute: codespace: %s, code: %d, log: %s", res.Codespace, res.Code, res.Log)
			}
		}
		return err
	}

	if res == nil {
		return fmt.Errorf("unexpected nil response from BroadcastTxSync")
	}

	// TODO: maybe we need to check if the node has tx indexing enabled?
	// if not, we need to find a new way to block until inclusion in a block
	go cc.waitForTx(asyncCtx, res.Hash, asyncTimeout, asyncCallbacks)

	return nil
}

// waitForTx waits for a transaction to be included in a block, logs success/fail, then invokes callback.
// This is intended to be called as an async goroutine.
func (cc *CosmosProvider) waitForTx(
	ctx context.Context,
	txHash []byte,
	waitTimeout time.Duration,
	callbacks []func(*RelayerTxResponse, error),
) {
	res, err := cc.waitForBlockInclusion(ctx, txHash, waitTimeout)
	if err != nil {
		if len(callbacks) > 0 {
			for _, cb := range callbacks {
				// Call each callback in order since waitForTx is already invoked asynchronously
				cb(nil, err)
			}
		}
		return
	}

	rlyResp := &RelayerTxResponse{
		Height:    res.Height,
		TxHash:    res.TxHash,
		Codespace: res.Codespace,
		Code:      res.Code,
		Data:      res.Data,
		Events:    parseEventsFromTxResponse(res),
	}

	// NOTE: error is nil, logic should use the returned error to determine if the
	// transaction was successfully executed.
	if res.Code != 0 {
		// Check for any registered SDK errors
		err := cc.sdkError(res.Codespace, res.Code)
		if err == nil {
			err = fmt.Errorf("transaction failed to execute: codespace: %s, code: %d, log: %s", res.Codespace, res.Code, res.RawLog)
		}
		if len(callbacks) > 0 {
			for _, cb := range callbacks {
				// Call each callback in order since waitForTx is already invoked asynchronously
				cb(nil, err)
			}
		}
		return
	}

	if len(callbacks) > 0 {
		for _, cb := range callbacks {
			// Call each callback in order since waitForTx is already invoked asynchronously
			cb(rlyResp, nil)
		}
	}
}

// waitForBlockInclusion will wait for a transaction to be included in a block, up to waitTimeout or context cancellation.
func (cc *CosmosProvider) waitForBlockInclusion(
	ctx context.Context,
	txHash []byte,
	waitTimeout time.Duration,
) (*sdk.TxResponse, error) {
	exitAfter := time.After(waitTimeout)
	for {
		select {
		case <-exitAfter:
			return nil, fmt.Errorf("timed out after: %d; %w", waitTimeout, ErrTimeoutAfterWaitingForTxBroadcast)
		// This fixed poll is fine because it's only for logging and updating prometheus metrics currently.
		case <-time.After(time.Millisecond * 100):
			res, err := cc.RPCClient.Tx(ctx, txHash, false)
			if err == nil {
				return cc.mkTxResult(res)
			}
			if strings.Contains(err.Error(), "transaction indexing is disabled") {
				return nil, fmt.Errorf("cannot determine success/failure of tx because transaction indexing is disabled on rpc url")
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// mkTxResult decodes a comet transaction into an SDK TxResponse.
func (cc *CosmosProvider) mkTxResult(resTx *coretypes.ResultTx) (*sdk.TxResponse, error) {
	txBz, err := cc.Cdc.TxConfig.TxDecoder()(resTx.Tx)
	if err != nil {
		return nil, err
	}

	p, ok := txBz.(intoAny)
	if !ok {
		return nil, fmt.Errorf("expecting a type implementing intoAny, got: %T", txBz)
	}

	return sdk.NewResponseResultTx(resTx, p.AsAny(), ""), nil
}

func sdkErrorToGRPCError(resp abci.ResponseQuery) error {
	switch resp.Code {
	case legacyerrors.ErrInvalidRequest.ABCICode():
		return status.Error(codes.InvalidArgument, resp.Log)
	case legacyerrors.ErrUnauthorized.ABCICode():
		return status.Error(codes.Unauthenticated, resp.Log)
	case legacyerrors.ErrKeyNotFound.ABCICode():
		return status.Error(codes.NotFound, resp.Log)
	default:
		return status.Error(codes.Unknown, resp.Log)
	}
}

// isQueryStoreWithProof expects a format like /<queryType>/<storeName>/<subPath>
// queryType must be "store" and subPath must be "key" to require a proof.
func isQueryStoreWithProof(path string) bool {
	if !strings.HasPrefix(path, "/") {
		return false
	}

	paths := strings.SplitN(path[1:], "/", 3)

	switch {
	case len(paths) != 3:
		return false
	case paths[0] != "store":
		return false
	case rootmulti.RequireProof("/" + paths[2]):
		return true
	}

	return false
}

// sdkError will return the Cosmos SDK registered error for a given codeSpace/code combo if registered, otherwise nil.
func (cc *CosmosProvider) sdkError(codeSpace string, code uint32) error {
	// ABCIError will return an error other than "unknown" if syncRes.Code is a registered error in syncRes.CodeSpace
	// This catches all the sdk errors https://github.com/cosmos/cosmos-sdk/blob/f10f5e5974d2ecbf9efc05bc0bfe1c99fdeed4b6/types/errors/errors.go
	err := errors.Unwrap(sdkerrors.ABCIError(codeSpace, code, "error broadcasting transaction"))
	if err == nil {
		return nil
	}

	if err.Error() != errUnknown {
		return err
	}
	return nil
}

func parseEventsFromTxResponse(resp *sdk.TxResponse) []RelayerEvent {
	var events []RelayerEvent

	if resp == nil {
		return events
	}

	for _, logs := range resp.Logs {
		for _, event := range logs.Events {
			attributes := make(map[string]string)
			for _, attribute := range event.Attributes {
				attributes[attribute.Key] = attribute.Value
			}
			events = append(events, RelayerEvent{
				EventType:  event.Type,
				Attributes: attributes,
			})
		}
	}

	// After SDK v0.50, indexed events are no longer provided in the logs on
	// transaction execution, the response events can be directly used
	if len(events) == 0 {
		for _, event := range resp.Events {
			attributes := make(map[string]string)
			for _, attribute := range event.Attributes {
				attributes[attribute.Key] = attribute.Value
			}
			events = append(events, RelayerEvent{
				EventType:  event.Type,
				Attributes: attributes,
			})
		}
	}

	return events
}

// handleAccountSequenceMismatchError will parse the error string, e.g.:
// "account sequence mismatch, expected 10, got 9: incorrect account sequence"
// and update the next account sequence with the expected value.
func (cc *CosmosProvider) handleAccountSequenceMismatchError(sequenceGuard *WalletState, err error) {
	if sequenceGuard == nil {
		panic("sequence guard not configured")
	}

	matches := accountSeqRegex.FindStringSubmatch(err.Error())
	if len(matches) == 0 {
		return
	}
	nextSeq, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return
	}
	sequenceGuard.NextAccountSequence = nextSeq
}

// SendMessagesToMempool simulates and broadcasts a transaction with the given msgs and memo.
// This method will return once the transaction has entered the mempool.
// In an async goroutine, will wait for the tx to be included in the block unless asyncCtx exits.
// If there is no error broadcasting, the asyncCallback will be called with success/failure of the wait for block inclusion.
func (cc *CosmosProvider) SendMessagesToMempool(
	ctx context.Context,
	msgs []RelayerMessage,
	memo string,
	asyncCtx context.Context,
	asyncCallbacks []func(*RelayerTxResponse, error),
) error {
	txSignerKey := cc.PCfg.Key

	sequenceGuard := ensureSequenceGuard(cc, txSignerKey)
	sequenceGuard.Mu.Lock()
	defer sequenceGuard.Mu.Unlock()

	txBytes, sequence, _, err := cc.buildMessages(ctx, msgs, memo, 0, txSignerKey, sequenceGuard)
	if err != nil {
		// Account sequence mismatch errors can happen on the simulated transaction also.
		if strings.Contains(err.Error(), legacyerrors.ErrWrongSequence.Error()) {
			cc.handleAccountSequenceMismatchError(sequenceGuard, err)
		}

		return err
	}

	if err := cc.broadcastTx(ctx, txBytes, asyncCtx, defaultBroadcastWaitTimeout, asyncCallbacks); err != nil {
		if strings.Contains(err.Error(), legacyerrors.ErrWrongSequence.Error()) {
			cc.handleAccountSequenceMismatchError(sequenceGuard, err)
		}

		return err
	}

	// we had a successful tx broadcast with this sequence, so update it to the next
	cc.updateNextAccountSequence(sequenceGuard, sequence+1)
	return nil
}

func (cc *CosmosProvider) updateNextAccountSequence(sequenceGuard *WalletState, seq uint64) {
	if seq > sequenceGuard.NextAccountSequence {
		sequenceGuard.NextAccountSequence = seq
	}
}

func (cc *CosmosProvider) buildMessages(
	ctx context.Context,
	msgs []RelayerMessage,
	memo string,
	gas uint64,
	txSignerKey string,
	sequenceGuard *WalletState,
) (
	txBytes []byte,
	sequence uint64,
	fees sdk.Coins,
	err error,
) {
	done := cc.SetSDKContext()
	defer done()

	cMsgs := CosmosMsgs(msgs...)

	txf, err := cc.PrepareFactory(cc.TxFactory(), txSignerKey)
	if err != nil {
		return nil, 0, sdk.Coins{}, err
	}

	if memo != "" {
		txf = txf.WithMemo(memo)
	}

	sequence = txf.Sequence()
	cc.updateNextAccountSequence(sequenceGuard, sequence)
	if sequence < sequenceGuard.NextAccountSequence {
		sequence = sequenceGuard.NextAccountSequence
		txf = txf.WithSequence(sequence)
	}

	adjusted := gas

	if gas == 0 {
		_, adjusted, err = cc.CalculateGas(ctx, txf, txSignerKey, cMsgs...)

		if err != nil {
			return nil, 0, sdk.Coins{}, err
		}
	}

	// Set the gas amount on the transaction factory
	txf = txf.WithGas(adjusted)

	// Build the transaction builder
	txb, err := txf.BuildUnsignedTx(cMsgs...)
	if err != nil {
		return nil, 0, sdk.Coins{}, err
	}

	if err = tx.Sign(ctx, txf, txSignerKey, txb, false); err != nil {
		return nil, 0, sdk.Coins{}, err
	}

	tx := txb.GetTx()
	fees = tx.GetFee()

	// Generate the transaction bytes
	txBytes, err = cc.Cdc.TxConfig.TxEncoder()(tx)
	if err != nil {
		return nil, 0, sdk.Coins{}, err
	}

	return txBytes, txf.Sequence(), fees, nil
}

// PrepareFactory mutates the tx factory with the appropriate account number, sequence number, and min gas settings.
func (cc *CosmosProvider) PrepareFactory(txf tx.Factory, signingKey string) (tx.Factory, error) {
	var (
		err      error
		from     sdk.AccAddress
		num, seq uint64
	)

	// Get key address and retry if fail
	if err = retry.Do(func() error {
		from, err = cc.GetKeyAddressForKey(signingKey)
		if err != nil {
			return err
		}
		return err
	}, rtyAtt, rtyDel, rtyErr); err != nil {
		return tx.Factory{}, err
	}

	cliCtx := client.Context{}.WithClient(cc.RPCClient).
		WithInterfaceRegistry(cc.Cdc.InterfaceRegistry).
		WithChainID(cc.PCfg.ChainID).
		WithCodec(cc.Cdc.Codec).
		WithFromAddress(from)

	// Set the account number and sequence on the transaction factory and retry if fail
	if err = retry.Do(func() error {
		if err = txf.AccountRetriever().EnsureExists(cliCtx, from); err != nil {
			return err
		}
		return err
	}, rtyAtt, rtyDel, rtyErr); err != nil {
		return txf, err
	}

	// TODO: why this code? this may potentially require another query when we don't want one
	initNum, initSeq := txf.AccountNumber(), txf.Sequence()
	if initNum == 0 || initSeq == 0 {
		if err = retry.Do(func() error {
			num, seq, err = txf.AccountRetriever().GetAccountNumberSequence(cliCtx, from)
			if err != nil {
				return err
			}
			return err
		}, rtyAtt, rtyDel, rtyErr); err != nil {
			return txf, err
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

	return txf, nil
}

// TxFactory instantiates a new tx factory with the appropriate configuration settings for this chain.
func (cc *CosmosProvider) TxFactory() tx.Factory {
	return tx.Factory{}.
		WithAccountRetriever(cc).
		WithChainID(cc.PCfg.ChainID).
		WithTxConfig(cc.Cdc.TxConfig).
		WithGasAdjustment(cc.PCfg.GasAdjustment).
		WithGasPrices(cc.PCfg.GasPrices).
		WithKeybase(cc.Keybase).
		WithSignMode(cc.PCfg.SignMode())
}

// SignMode returns the SDK sign mode type reflective of the specified sign mode in the config file.
func (pc CosmosProviderConfig) SignMode() signing.SignMode {
	signMode := signing.SignMode_SIGN_MODE_UNSPECIFIED
	switch pc.SignModeStr {
	case "direct":
		signMode = signing.SignMode_SIGN_MODE_DIRECT
	case "amino-json":
		signMode = signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON
	}
	return signMode
}

// CalculateGas simulates a tx to generate the appropriate gas settings before broadcasting a tx.
func (cc *CosmosProvider) CalculateGas(ctx context.Context, txf tx.Factory, signingKey string, msgs ...sdk.Msg) (txtypes.SimulateResponse, uint64, error) {
	keyInfo, err := cc.Keybase.Key(signingKey)
	if err != nil {
		return txtypes.SimulateResponse{}, 0, err
	}

	var txBytes []byte
	if err := retry.Do(func() error {
		var err error
		txBytes, err = BuildSimTx(keyInfo, txf, msgs...)
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

// BuildSimTx creates an unsigned tx with an empty single signature and returns
// the encoded transaction or an error if the unsigned transaction cannot be built.
func BuildSimTx(info *keyring.Record, txf tx.Factory, msgs ...sdk.Msg) ([]byte, error) {
	txb, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, err
	}

	var pk cryptotypes.PubKey // use default public key type

	pk, err = info.GetPubKey()
	if err != nil {
		return nil, err
	}

	// Create an empty signature literal as the ante handler will populate with a
	// sentinel pubKey.
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

// AdjustEstimatedGas adjusts the estimated gas usage by multiplying it by the gas adjustment factor
// and return estimated gas is higher than max gas error. If the gas usage is zero, the adjusted gas
// is also zero.
func (cc *CosmosProvider) AdjustEstimatedGas(gasUsed uint64) (uint64, error) {
	if gasUsed == 0 {
		return gasUsed, nil
	}
	if cc.PCfg.MaxGasAmount > 0 && gasUsed > cc.PCfg.MaxGasAmount {
		return 0, fmt.Errorf("estimated gas %d is higher than max gas %d", gasUsed, cc.PCfg.MaxGasAmount)
	}
	gas := cc.PCfg.GasAdjustment * float64(gasUsed)
	if math.IsInf(gas, 1) {
		return 0, fmt.Errorf("infinite gas used")
	}
	return uint64(gas), nil
}

// protoTxProvider is a type which can provide a proto transaction. It is a
// workaround to get access to the wrapper TxBuilder's method GetProtoTx().
type protoTxProvider interface {
	GetProtoTx() *txtypes.Tx
}
