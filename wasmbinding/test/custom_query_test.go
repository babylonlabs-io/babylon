package wasmbinding

import (
	"encoding/json"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/babylon/app"
	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/babylon/wasmbinding/bindings"
)

// TODO consider doing it by enviromental variables as currently it may fail on some
// weird architectures
func getArtifactPath() string {
	if runtime.GOARCH == "amd64" {
		return "../testdata/artifacts/testdata.wasm"
	} else if runtime.GOARCH == "arm64" {
		return "../testdata/artifacts/testdata-aarch64.wasm"
	} else {
		panic("Unsupported architecture")
	}
}

var pathToContract = getArtifactPath()

func TestQueryEpoch(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	contractAddress := deployTestContract(t, ctx, babylonApp, acc, pathToContract)

	query := bindings.BabylonQuery{
		Epoch: &struct{}{},
	}
	resp := bindings.CurrentEpochResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp)
	require.Equal(t, resp.Epoch, uint64(1))

	newEpoch := babylonApp.EpochingKeeper.IncEpoch(ctx)

	resp = bindings.CurrentEpochResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp)
	require.Equal(t, resp.Epoch, newEpoch.EpochNumber)
}

func TestFinalizedEpoch(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	// babylonApp.ZoneConciergeKeeper
	contractAddress := deployTestContract(t, ctx, babylonApp, acc, pathToContract)

	query := bindings.BabylonQuery{
		LatestFinalizedEpochInfo: &struct{}{},
	}

	// Only epoch 0 is finalised at genesis
	resp := bindings.LatestFinalizedEpochInfoResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp)
	require.Equal(t, resp.EpochInfo.EpochNumber, uint64(0))
	require.Equal(t, resp.EpochInfo.LastBlockHeight, uint64(0))

	epoch := babylonApp.EpochingKeeper.InitEpoch(ctx)
	babylonApp.CheckpointingKeeper.SetCheckpointFinalized(ctx, epoch.EpochNumber)

	resp = bindings.LatestFinalizedEpochInfoResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp)
	require.Equal(t, resp.EpochInfo.EpochNumber, epoch.EpochNumber)
	require.Equal(t, resp.EpochInfo.LastBlockHeight, epoch.GetLastBlockHeight())
}

func TestQueryBtcTip(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	contractAddress := deployTestContract(t, ctx, babylonApp, acc, pathToContract)

	query := bindings.BabylonQuery{
		BtcTip: &struct{}{},
	}

	resp := bindings.BtcTipResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp)

	tip := babylonApp.BTCLightClientKeeper.GetTipInfo(ctx)
	tipAsInfo := bindings.AsBtcBlockHeaderInfo(tip)

	require.Equal(t, resp.HeaderInfo.Height, tip.Height)
	require.Equal(t, tipAsInfo, resp.HeaderInfo)
}

func TestQueryBtcBase(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	contractAddress := deployTestContract(t, ctx, babylonApp, acc, pathToContract)

	query := bindings.BabylonQuery{
		BtcBaseHeader: &struct{}{},
	}

	resp := bindings.BtcBaseHeaderResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp)

	base := babylonApp.BTCLightClientKeeper.GetBaseBTCHeader(ctx)
	baseAsInfo := bindings.AsBtcBlockHeaderInfo(base)

	require.Equal(t, baseAsInfo, resp.HeaderInfo)
}

func TestQueryBtcByHash(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	contractAddress := deployTestContract(t, ctx, babylonApp, acc, pathToContract)
	tip := babylonApp.BTCLightClientKeeper.GetTipInfo(ctx)

	query := bindings.BabylonQuery{
		BtcHeaderByHash: &bindings.BtcHeaderByHash{
			Hash: tip.Hash.String(),
		},
	}

	headerAsInfo := bindings.AsBtcBlockHeaderInfo(tip)
	resp := bindings.BtcHeaderQueryResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp)

	require.Equal(t, resp.HeaderInfo, headerAsInfo)
}

func TestQueryBtcByNumber(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	contractAddress := deployTestContract(t, ctx, babylonApp, acc, pathToContract)
	tip := babylonApp.BTCLightClientKeeper.GetTipInfo(ctx)

	query := bindings.BabylonQuery{
		BtcHeaderByHeight: &bindings.BtcHeaderByHeight{
			Height: tip.Height,
		},
	}

	headerAsInfo := bindings.AsBtcBlockHeaderInfo(tip)
	resp := bindings.BtcHeaderQueryResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, query, &resp)

	require.Equal(t, resp.HeaderInfo, headerAsInfo)
}

func TestQueryNonExistingHeader(t *testing.T) {
	acc := RandomAccountAddress()
	babylonApp, ctx := SetupAppWithContext(t)
	FundAccount(t, ctx, babylonApp, acc)

	contractAddress := deployTestContract(t, ctx, babylonApp, acc, pathToContract)

	queryNonExisitingHeight := bindings.BabylonQuery{
		BtcHeaderByHeight: &bindings.BtcHeaderByHeight{
			Height: 1,
		},
	}
	resp := bindings.BtcHeaderQueryResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, queryNonExisitingHeight, &resp)
	require.Nil(t, resp.HeaderInfo)

	// Random source for the generation of BTC hash
	r := rand.New(rand.NewSource(time.Now().Unix()))
	queryNonExisitingHash := bindings.BabylonQuery{
		BtcHeaderByHash: &bindings.BtcHeaderByHash{
			Hash: datagen.GenRandomBtcdHash(r).String(),
		},
	}
	resp1 := bindings.BtcHeaderQueryResponse{}
	queryCustom(t, ctx, babylonApp, contractAddress, queryNonExisitingHash, &resp1)
	require.Nil(t, resp1.HeaderInfo)
}

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

func deployTestContract(
	t *testing.T,
	ctx sdk.Context,
	bbn *app.BabylonApp,
	deployer sdk.AccAddress,
	codePath string,
) sdk.AccAddress {

	codeId, _ := StoreTestCodeCode(t, ctx, bbn, deployer, codePath)

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
