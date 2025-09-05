package precompiles

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	sdkheader "cosmossdk.io/core/header"
	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/server/config"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/app/params"
)

type BaseTestSuite struct {
	suite.Suite

	App *app.BabylonApp
	Ctx sdk.Context

	QueryClientBank    banktypes.QueryClient
	QueryClientStaking stakingtypes.QueryClient
}

// SetupApp initializes a fresh Babylon app and context
func (suite *BaseTestSuite) SetupApp(t *testing.T) {
	suite.App = app.Setup(t, false)
	header := cmtproto.Header{Height: 1, Time: time.Now().UTC()}
	ctx := suite.App.BaseApp.NewContextLegacy(false, header)
	if vals, err := suite.App.StakingKeeper.GetAllValidators(ctx); err == nil && len(vals) > 0 {
		if pk, pkErr := vals[0].ConsPubKey(); pkErr == nil {
			consAddr := sdk.ConsAddress(pk.Address().Bytes())
			h := ctx.BlockHeader()
			h.ProposerAddress = consAddr.Bytes()
			ctx = ctx.WithBlockHeader(h)
		}
	}
	suite.Ctx = ctx
	suite.Ctx = suite.Ctx.WithHeaderInfo(sdkheader.Info{Height: suite.Ctx.BlockHeader().Height, Time: suite.Ctx.BlockHeader().Time})

	queryHelper := baseapp.NewQueryServerTestHelper(suite.Ctx, suite.App.InterfaceRegistry())
	banktypes.RegisterQueryServer(queryHelper, suite.App.BankKeeper)
	suite.QueryClientBank = banktypes.NewQueryClient(queryHelper)
	stakingtypes.RegisterQueryServer(queryHelper, stakingkeeper.NewQuerier(suite.App.StakingKeeper))
	suite.QueryClientStaking = stakingtypes.NewQueryClient(queryHelper)
}

// InitAndFundEVMAccount creates an auth account for the given EVM private key and funds it.
func (suite *BaseTestSuite) InitAndFundEVMAccount(priv *ethsecp256k1.PrivKey, amount sdkmath.Int) (sdk.AccAddress, error) {
	addr := sdk.AccAddress(priv.PubKey().Address().Bytes())
	// Create account if not exists
	if suite.App.AccountKeeper.GetAccount(suite.Ctx, addr) == nil {
		acc := suite.App.AccountKeeper.NewAccountWithAddress(suite.Ctx, addr)
		suite.App.AccountKeeper.SetAccount(suite.Ctx, acc)
	}
	// Mint and send funds
	coins := sdk.NewCoins(sdk.NewCoin(params.BaseCoinUnit, amount))
	if err := suite.App.BankKeeper.MintCoins(suite.Ctx, minttypes.ModuleName, coins); err != nil {
		return nil, err
	}
	if err := suite.App.BankKeeper.SendCoinsFromModuleToAccount(suite.Ctx, minttypes.ModuleName, addr, coins); err != nil {
		return nil, err
	}
	return addr, nil
}

// TODO: DeployContract does not return *evmtypes.MsgEthereumTxResponse,
// which breaks consistency with other methods.
func (suite *BaseTestSuite) DeployContract(
	priv *ethsecp256k1.PrivKey,
	contractBin []byte,
	contractABI abi.ABI,
	args ...interface{},
) (common.Address, abi.ABI, error) {
	return suite.DeployContractWithValue(priv, nil, contractBin, contractABI, args...)
}

func (suite *BaseTestSuite) DeployContractWithValue(
	priv *ethsecp256k1.PrivKey,
	value *big.Int,
	contractBin []byte,
	contractABI abi.ABI,
	args ...interface{},
) (common.Address, abi.ABI, error) {
	chainID := evmtypes.GetEthChainConfig().ChainID

	// make calldata
	ctorArgs, err := contractABI.Pack("", args...)
	if err != nil {
		return common.Address{}, abi.ABI{}, err
	}
	calldata := make([]byte, len(contractBin)+len(ctorArgs))
	copy(calldata[:len(contractBin)], contractBin)
	copy(calldata[len(contractBin):], ctorArgs)

	addr := sdk.AccAddress(priv.PubKey().Address().Bytes())
	nonce, err := suite.App.AccountKeeper.GetSequence(suite.Ctx, addr)
	if err != nil {
		return common.Address{}, abi.ABI{}, err
	}

	// send evm tx
	evmAddr := common.Address(priv.PubKey().Address().Bytes())
	txArgs := evmtypes.TransactionArgs{
		From:    &evmAddr,
		Value:   (*hexutil.Big)(value),
		Nonce:   (*hexutil.Uint64)(&nonce),
		Data:    (*hexutil.Bytes)(&calldata),
		ChainID: (*hexutil.Big)(chainID),
	}
	evmRes, err := suite.SendEthTransaction(priv, txArgs)
	if err != nil {
		return common.Address{}, abi.ABI{}, err
	}
	if evmRes.Failed() {
		return common.Address{}, abi.ABI{}, fmt.Errorf("deploy transaction failed: %v", evmRes.VmError)
	}

	contractAddr := crypto.CreateAddress(evmAddr, nonce)
	return contractAddr, contractABI, nil
}

func (suite *BaseTestSuite) CallContract(
	priv *ethsecp256k1.PrivKey,
	contractAddr common.Address,
	contractABI abi.ABI,
	methodName string,
	args ...interface{},
) (*evmtypes.MsgEthereumTxResponse, error) {
	return suite.CallContractWithValue(
		priv,
		nil,
		contractAddr,
		contractABI,
		methodName,
		args...,
	)
}

func (suite *BaseTestSuite) CallContractWithValue(
	priv *ethsecp256k1.PrivKey,
	value *big.Int,
	contractAddr common.Address,
	contractABI abi.ABI,
	methodName string,
	args ...interface{},
) (*evmtypes.MsgEthereumTxResponse, error) {
	chainID := evmtypes.GetEthChainConfig().ChainID

	// make calldata
	calldata, err := contractABI.Pack(methodName, args...)
	if err != nil {
		return nil, errorsmod.Wrap(err, "fail to pack contract method and args")
	}

	addr := sdk.AccAddress(priv.PubKey().Address().Bytes())
	nonce, err := suite.App.AccountKeeper.GetSequence(suite.Ctx, addr)
	if err != nil {
		return nil, errorsmod.Wrap(err, "fail to get sequence")
	}
	evmAddr := common.Address(priv.PubKey().Address().Bytes())

	// send evm tx
	txArgs := evmtypes.TransactionArgs{
		From:    &evmAddr,
		To:      &contractAddr,
		Value:   (*hexutil.Big)(value),
		Nonce:   (*hexutil.Uint64)(&nonce),
		Data:    (*hexutil.Bytes)(&calldata),
		ChainID: (*hexutil.Big)(chainID),
	}
	return suite.SendEthTransaction(priv, txArgs)
}

func (suite *BaseTestSuite) SendEthValue(
	priv *ethsecp256k1.PrivKey,
	toAddr common.Address,
	value *big.Int,
) (*evmtypes.MsgEthereumTxResponse, error) {
	chainID := evmtypes.GetEthChainConfig().ChainID

	addr := sdk.AccAddress(priv.PubKey().Address().Bytes())
	nonce, err := suite.App.AccountKeeper.GetSequence(suite.Ctx, addr)
	if err != nil {
		return nil, errorsmod.Wrap(err, "fail to get sequence")
	}
	evmAddr := common.Address(priv.PubKey().Address().Bytes())

	// send evm tx
	txArgs := evmtypes.TransactionArgs{
		From:    &evmAddr,
		To:      &toAddr,
		Value:   (*hexutil.Big)(value),
		Nonce:   (*hexutil.Uint64)(&nonce),
		ChainID: (*hexutil.Big)(chainID),
	}
	return suite.SendEthTransaction(priv, txArgs)
}

func (suite *BaseTestSuite) SendEthTransaction(
	priv *ethsecp256k1.PrivKey,
	txArgs evmtypes.TransactionArgs,
) (*evmtypes.MsgEthereumTxResponse, error) {
	// make MsgEthereumTx
	_, txBytes, err := suite.MakeEthTx(priv, txArgs)
	if err != nil {
		return nil, err
	}

	// commit
	res := suite.Commit([][]byte{txBytes})
	if res.TxResults[0].Code != 0 {
		return nil, fmt.Errorf("transaction failed: %v", res.TxResults[0].Log)
	}

	evmRes, err := evmtypes.DecodeTxResponse(res.TxResults[0].Data)
	if err != nil {
		return nil, err
	}

	return evmRes, nil
}

func (suite *BaseTestSuite) QueryContract(
	contractAddr common.Address,
	contractABI abi.ABI,
	methodName string,
	args ...interface{},
) (*evmtypes.MsgEthereumTxResponse, error) {
	chainID := evmtypes.GetEthChainConfig().ChainID

	calldata, err := contractABI.Pack(methodName, args...)
	if err != nil {
		return nil, err
	}

	txArgs := evmtypes.TransactionArgs{
		To:   &contractAddr,
		Data: (*hexutil.Bytes)(&calldata),
	}
	argsJSON, err := json.Marshal(txArgs)
	if err != nil {
		return nil, err
	}

	queryCtx := suite.Ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	evmRes, err := suite.App.AppKeepers.EVMKeeper.EthCall(queryCtx, &evmtypes.EthCallRequest{
		Args:            argsJSON,
		ProposerAddress: suite.Ctx.BlockHeader().ProposerAddress,
		ChainId:         chainID.Int64(),
		GasCap:          config.DefaultGasCap,
	})
	if err != nil {
		return nil, err
	}
	if evmRes.Failed() {
		return nil, fmt.Errorf("transaction failed: %v", evmRes.VmError)
	}

	return evmRes, nil
}

// AdvanceToNextEpoch advances the chain until the epoch number increases.
func (suite *BaseTestSuite) AdvanceToNextEpoch() {
	for !suite.App.EpochingKeeper.GetEpoch(suite.Ctx).IsLastBlock(suite.Ctx) {
		suite.Commit(nil)
	}
	suite.Commit(nil)
}

func (suite *BaseTestSuite) MakeEthTx(
	priv *ethsecp256k1.PrivKey,
	txArgs evmtypes.TransactionArgs,
) (*evmtypes.MsgEthereumTx, []byte, error) {
	chainID := txArgs.ChainID.ToInt()

	// estimate gas
	argsJSON, err := json.Marshal(&txArgs)
	if err != nil {
		return nil, nil, err
	}
	estimateGasRes, err := suite.App.AppKeepers.EVMKeeper.EstimateGas(suite.Ctx, &evmtypes.EthCallRequest{
		Args:            argsJSON,
		ProposerAddress: suite.Ctx.BlockHeader().ProposerAddress,
		ChainId:         chainID.Int64(),
		GasCap:          config.DefaultGasCap,
	})
	if err != nil {
		return nil, nil, err
	}
	if estimateGasRes.VmError != "" {
		return nil, nil, fmt.Errorf("transaction failed: %v", estimateGasRes.VmError)
	}
	gasLimit := estimateGasRes.Gas + estimateGasRes.Gas*10/100 // 10% buffer

	calldata := []byte{}
	if txArgs.Data != nil {
		calldata = *txArgs.Data
	}

	value := big.NewInt(0)
	if txArgs.Value != nil {
		value = txArgs.Value.ToInt()
	}

	// make evm tx
	tx := evmtypes.NewTx(
		&evmtypes.EvmTxArgs{
			Nonce:             uint64(*txArgs.Nonce),
			GasLimit:          gasLimit,
			Input:             calldata,
			GasFeeCap:         suite.App.AppKeepers.EVMKeeper.GetBaseFee(suite.Ctx),
			ChainID:           chainID,
			Amount:            value,
			GasTipCap:         nil,
			To:                txArgs.To,
			Accesses:          &ethtypes.AccessList{},
			AuthorizationList: txArgs.AuthorizationList,
		},
	)
	tx.From = priv.PubKey().Address().Bytes()

	// MsgEthereumTx -> TxBytes
	return suite.prepareEthTx(priv, chainID, tx)
}

func (suite *BaseTestSuite) prepareEthTx(
	priv *ethsecp256k1.PrivKey,
	chainID *big.Int,
	msgEthereumTx *evmtypes.MsgEthereumTx,
) (*evmtypes.MsgEthereumTx, []byte, error) {
	ethSigner := ethtypes.LatestSignerForChainID(chainID)
	err := msgEthereumTx.Sign(ethSigner, NewSigner(priv))
	if err != nil {
		return nil, nil, err
	}

	txConfig := suite.App.TxConfig()
	evmDenom := evmtypes.GetEVMCoinDenom()

	tx, err := msgEthereumTx.BuildTx(txConfig.NewTxBuilder(), evmDenom)
	if err != nil {
		return nil, nil, err
	}

	// bz are bytes to be broadcasted over the network
	bz, err := txConfig.TxEncoder()(tx)
	if err != nil {
		return nil, nil, err
	}

	sdkTx, err := txConfig.TxDecoder()(bz)
	if err != nil {
		return nil, nil, err
	}

	return sdkTx.GetMsgs()[0].(*evmtypes.MsgEthereumTx), bz, nil
}

func (suite *BaseTestSuite) MakeCosmosTx(priv *ethsecp256k1.PrivKey, msgs ...sdk.Msg) ([]byte, sdk.Tx, error) {
	txConfig := suite.App.TxConfig()
	txBuilder := txConfig.NewTxBuilder()

	txBuilder.SetGasLimit(1000000)
	fees := sdk.NewCoins(sdk.NewCoin(params.BaseEVMDenom, sdkmath.NewInt(1000000)))
	txBuilder.SetFeeAmount(fees)

	err := txBuilder.SetMsgs(msgs...)
	if err != nil {
		return nil, nil, err
	}

	addr := sdk.AccAddress(priv.PubKey().Address().Bytes())
	seq, err := suite.App.AccountKeeper.GetSequence(suite.Ctx, addr)
	if err != nil {
		return nil, nil, err
	}

	defaultMode, err := authsigning.APISignModeToInternal(txConfig.SignModeHandler().DefaultMode())
	if err != nil {
		return nil, nil, err
	}

	// First round: we gather all the signer infos. We use the "set empty
	// signature" hack to do that.
	sigV2 := signing.SignatureV2{
		PubKey: priv.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  defaultMode,
			Signature: nil,
		},
		Sequence: seq,
	}

	sigsV2 := []signing.SignatureV2{sigV2}

	err = txBuilder.SetSignatures(sigsV2...)
	if err != nil {
		return nil, nil, err
	}

	// Second round: all signer infos are set, so each signer can sign.
	accNumber := suite.App.AccountKeeper.GetAccount(suite.Ctx, addr).GetAccountNumber()
	signerData := authsigning.SignerData{
		ChainID:       suite.Ctx.ChainID(),
		AccountNumber: accNumber,
		Sequence:      seq,
	}
	sigV2, err = tx.SignWithPrivKey(
		context.Background(),
		defaultMode, signerData,
		txBuilder, priv, txConfig,
		seq,
	)
	if err != nil {
		return nil, nil, err
	}

	sigsV2 = []signing.SignatureV2{sigV2}
	err = txBuilder.SetSignatures(sigsV2...)
	if err != nil {
		return nil, nil, err
	}

	tx := txBuilder.GetTx()
	bz, err := txConfig.TxEncoder()(tx)
	if err != nil {
		return nil, nil, err
	}

	return bz, tx, nil
}

// Commit and begin new block
func (suite *BaseTestSuite) Commit(txs [][]byte) *abci.ResponseFinalizeBlock {
	return suite.CommitAfter(txs, time.Second*0)
}

func (suite *BaseTestSuite) CommitAfter(txs [][]byte, t time.Duration) *abci.ResponseFinalizeBlock {
	header := suite.Ctx.BlockHeader()
	header.Time = header.Time.Add(t)
	// Run BeginBlocker for this height to trigger epoch transitions
	if _, err := suite.App.BeginBlocker(suite.Ctx); err != nil {
		panic(err)
	}
	res, _ := suite.App.FinalizeBlock(&abci.RequestFinalizeBlock{
		Txs:             txs,
		Height:          header.Height,
		Time:            header.Time,
		ProposerAddress: suite.Ctx.BlockHeader().ProposerAddress,
	})
	_, err := suite.App.Commit()
	if err != nil {
		panic(err)
	}

	header.Height++
	suite.Ctx = suite.App.BaseApp.NewUncachedContext(false, header)
	suite.Ctx = suite.Ctx.WithHeaderInfo(sdkheader.Info{Height: suite.Ctx.BlockHeader().Height, Time: suite.Ctx.BlockHeader().Time})

	queryHelper := baseapp.NewQueryServerTestHelper(suite.Ctx, suite.App.InterfaceRegistry())

	banktypes.RegisterQueryServer(queryHelper, suite.App.BankKeeper)
	suite.QueryClientBank = banktypes.NewQueryClient(queryHelper)
	stakingtypes.RegisterQueryServer(queryHelper, stakingkeeper.NewQuerier(suite.App.StakingKeeper))
	suite.QueryClientStaking = stakingtypes.NewQueryClient(queryHelper)
	return res
}
