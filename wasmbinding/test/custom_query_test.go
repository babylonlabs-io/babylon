package wasmbinding

import (
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"runtime"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v2/types"
	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/wasmbinding/bindings"
)

// TODO consider doing it by environmental variables as currently it may fail on some
// weird architectures
func getArtifactPath() string {
	switch runtime.GOARCH {
	case "amd64":
		return "../testdata/artifacts/testdata.wasm"
	case "arm64":
		return "../testdata/artifacts/testdata-aarch64.wasm"
	default:
		panic("Unsupported architecture")
	}
}

var pathToContract = getArtifactPath()

func TestQueryEpoch(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	contractAddress := DeployTestContract(t, ctx, babylonApp, acc, pathToContract)

	query := bindings.BabylonQuery{
		Epoch: &struct{}{},
	}
	resp := bindings.CurrentEpochResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp, nil)
	require.Equal(t, resp.Epoch, uint64(1))

	newEpoch := babylonApp.EpochingKeeper.IncEpoch(ctx)

	resp = bindings.CurrentEpochResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp, nil)
	require.Equal(t, resp.Epoch, newEpoch.EpochNumber)
}

func TestFinalizedEpoch(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	// babylonApp.ZoneConciergeKeeper
	contractAddress := DeployTestContract(t, ctx, babylonApp, acc, pathToContract)

	query := bindings.BabylonQuery{
		LatestFinalizedEpochInfo: &struct{}{},
	}

	// Only epoch 0 is finalised at genesis
	resp := bindings.LatestFinalizedEpochInfoResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp, nil)
	require.Equal(t, resp.EpochInfo.EpochNumber, uint64(0))
	require.Equal(t, resp.EpochInfo.LastBlockHeight, uint64(0))

	err := babylonApp.EpochingKeeper.InitEpoch(ctx, nil)
	require.NoError(t, err)
	expEpochNum := uint64(0)
	babylonApp.CheckpointingKeeper.SetCheckpointFinalized(ctx, 0)

	resp = bindings.LatestFinalizedEpochInfoResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp, nil)
	require.Equal(t, resp.EpochInfo.EpochNumber, expEpochNum)
	require.Equal(t, resp.EpochInfo.LastBlockHeight, expEpochNum)
}

func TestQueryBtcTip(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	contractAddress := DeployTestContract(t, ctx, babylonApp, acc, pathToContract)

	query := bindings.BabylonQuery{
		BtcTip: &struct{}{},
	}

	resp := bindings.BtcTipResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp, nil)

	tip := babylonApp.BTCLightClientKeeper.GetTipInfo(ctx)
	tipAsInfo := bindings.AsBtcBlockHeaderInfo(tip)

	require.Equal(t, resp.HeaderInfo.Height, tip.Height)
	require.Equal(t, tipAsInfo, resp.HeaderInfo)
}

func TestQueryBtcBase(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	contractAddress := DeployTestContract(t, ctx, babylonApp, acc, pathToContract)

	query := bindings.BabylonQuery{
		BtcBaseHeader: &struct{}{},
	}

	resp := bindings.BtcBaseHeaderResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp, nil)

	base := babylonApp.BTCLightClientKeeper.GetBaseBTCHeader(ctx)
	baseAsInfo := bindings.AsBtcBlockHeaderInfo(base)

	require.Equal(t, baseAsInfo, resp.HeaderInfo)
}

func TestQueryBtcByHash(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	contractAddress := DeployTestContract(t, ctx, babylonApp, acc, pathToContract)
	tip := babylonApp.BTCLightClientKeeper.GetTipInfo(ctx)

	query := bindings.BabylonQuery{
		BtcHeaderByHash: &bindings.BtcHeaderByHash{
			Hash: tip.Hash.String(),
		},
	}

	headerAsInfo := bindings.AsBtcBlockHeaderInfo(tip)
	resp := bindings.BtcHeaderQueryResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp, nil)

	require.Equal(t, resp.HeaderInfo, headerAsInfo)
}

func TestQueryBtcByNumber(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	contractAddress := DeployTestContract(t, ctx, babylonApp, acc, pathToContract)
	tip := babylonApp.BTCLightClientKeeper.GetTipInfo(ctx)

	query := bindings.BabylonQuery{
		BtcHeaderByHeight: &bindings.BtcHeaderByHeight{
			Height: tip.Height,
		},
	}

	headerAsInfo := bindings.AsBtcBlockHeaderInfo(tip)
	resp := bindings.BtcHeaderQueryResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp, nil)

	require.Equal(t, resp.HeaderInfo, headerAsInfo)
}

func TestQueryNonExistingHeader(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	contractAddress := DeployTestContract(t, ctx, babylonApp, acc, pathToContract)

	queryNonExisitingHeight := bindings.BabylonQuery{
		BtcHeaderByHeight: &bindings.BtcHeaderByHeight{
			Height: 1,
		},
	}
	resp := bindings.BtcHeaderQueryResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, queryNonExisitingHeight, &resp, nil)
	require.Nil(t, resp.HeaderInfo)

	// Random source for the generation of BTC hash
	r := rand.New(rand.NewSource(time.Now().Unix()))
	queryNonExisitingHash := bindings.BabylonQuery{
		BtcHeaderByHash: &bindings.BtcHeaderByHash{
			Hash: datagen.GenRandomBtcdHash(r).String(),
		},
	}
	resp1 := bindings.BtcHeaderQueryResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, queryNonExisitingHash, &resp1, errors.New("Generic error: Querier contract error: codespace: btclightclient, code: 1100: query wasm contract failed"))
	require.Nil(t, resp1.HeaderInfo)
}

//nolint:unused
func setupAppWithContext(t *testing.T) (*app.BabylonApp, sdk.Context) {
	return setupAppWithContextAndCustomHeight(t, 1)
}

//nolint:unused
func setupAppWithContextAndCustomHeight(t *testing.T, height int64) (*app.BabylonApp, sdk.Context) {
	babylonApp := app.Setup(t, false)
	ctx := babylonApp.BaseApp.NewContext(false).
		WithBlockHeader(cmtproto.Header{Height: height, Time: time.Now().UTC()})
	return babylonApp, ctx
}

//nolint:unused
func keyPubAddr() (crypto.PrivKey, crypto.PubKey, sdk.AccAddress) {
	key := ed25519.GenPrivKey()
	pub := key.PubKey()
	addr := sdk.AccAddress(pub.Address())
	return key, pub, addr
}

//nolint:unused
func randomAccountAddress() sdk.AccAddress {
	_, _, addr := keyPubAddr()
	return addr
}

//nolint:unused
func mintCoinsTo(
	bankKeeper bankkeeper.Keeper,
	ctx sdk.Context,
	addr sdk.AccAddress,
	amounts sdk.Coins) error {
	if err := bankKeeper.MintCoins(ctx, minttypes.ModuleName, amounts); err != nil {
		return err
	}

	return bankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, amounts)
}

//nolint:unused
func fundAccount(
	t *testing.T,
	ctx sdk.Context,
	bbn *app.BabylonApp,
	acc sdk.AccAddress) {
	err := mintCoinsTo(bbn.BankKeeper, ctx, acc, sdk.NewCoins(
		sdk.NewCoin("ubbn", math.NewInt(10000000000)),
	))
	require.NoError(t, err)
}

//nolint:unused
func storeTestCodeCode(
	t *testing.T,
	ctx sdk.Context,
	babylonApp *app.BabylonApp,
	addr sdk.AccAddress,
	codePath string,
) (uint64, []byte) {
	wasmCode, err := os.ReadFile(codePath)

	require.NoError(t, err)
	permKeeper := keeper.NewPermissionedKeeper(babylonApp.WasmKeeper, keeper.DefaultAuthorizationPolicy{})
	id, checksum, err := permKeeper.Create(ctx, addr, wasmCode, nil)
	require.NoError(t, err)
	return id, checksum
}

//nolint:unused
func instantiateExampleContract(
	t *testing.T,
	ctx sdk.Context,
	bbn *app.BabylonApp,
	funder sdk.AccAddress,
	codeId uint64,
) sdk.AccAddress {
	initMsgBz := []byte("{}")
	contractKeeper := keeper.NewDefaultPermissionKeeper(bbn.WasmKeeper)
	addr, _, err := contractKeeper.Instantiate(ctx, codeId, funder, funder, initMsgBz, "demo contract", nil)
	require.NoError(t, err)
	return addr
}

//nolint:unused
func deployTestContract(
	t *testing.T,
	ctx sdk.Context,
	bbn *app.BabylonApp,
	deployer sdk.AccAddress,
	codePath string,
) sdk.AccAddress {
	codeId, _ := storeTestCodeCode(t, ctx, bbn, deployer, codePath)

	contractAddr := instantiateExampleContract(t, ctx, bbn, deployer, codeId)

	return contractAddr
}

type ExampleQuery struct {
	Chain *ChainRequest `json:"chain,omitempty"`
}

type ChainRequest struct {
	Request wasmvmtypes.QueryRequest `json:"request"`
}

type ChainResponse struct {
	Data []byte `json:"data"`
}

func queryCustom(
	t *testing.T,
	ctx sdk.Context,
	bbn *app.BabylonApp,
	contract sdk.AccAddress,
	request bindings.BabylonQuery,
	response interface{},
	expectedError error,
) {
	msgBz, err := json.Marshal(request)
	require.NoError(t, err)

	query := ExampleQuery{
		Chain: &ChainRequest{
			Request: wasmvmtypes.QueryRequest{Custom: msgBz},
		},
	}
	queryBz, err := json.Marshal(query)
	require.NoError(t, err)

	resBz, err := bbn.WasmKeeper.QuerySmart(ctx, contract, queryBz)
	if expectedError != nil {
		require.EqualError(t, expectedError, err.Error())
		return
	}

	require.NoError(t, err)
	var resp ChainResponse
	err = json.Unmarshal(resBz, &resp)
	require.NoError(t, err)
	err = json.Unmarshal(resp.Data, response)
	require.NoError(t, err)
}

//nolint:unused
func queryCustomErr(
	t *testing.T,
	ctx sdk.Context,
	bbn *app.BabylonApp,
	contract sdk.AccAddress,
	request bindings.BabylonQuery,
) {
	msgBz, err := json.Marshal(request)
	require.NoError(t, err)

	query := ExampleQuery{
		Chain: &ChainRequest{
			Request: wasmvmtypes.QueryRequest{Custom: msgBz},
		},
	}
	queryBz, err := json.Marshal(query)
	require.NoError(t, err)

	_, err = bbn.WasmKeeper.QuerySmart(ctx, contract, queryBz)
	require.Error(t, err)
}
