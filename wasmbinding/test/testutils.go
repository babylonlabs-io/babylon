package wasmbinding

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/CosmWasm/wasmd/x/wasm/keeper"
	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/stretchr/testify/require"
)

func KeyPubAddr() (crypto.PrivKey, crypto.PubKey, sdk.AccAddress) {
	key := ed25519.GenPrivKey()
	pub := key.PubKey()
	addr := sdk.AccAddress(pub.Address())
	return key, pub, addr
}

func RandomAccountAddress() sdk.AccAddress {
	_, _, addr := KeyPubAddr()
	return addr
}

func SetupAppWithContext(t *testing.T) (*app.BabylonApp, sdk.Context) {
	return SetupAppWithContextAndCustomHeight(t, 1)
}

func SetupAppWithContextAndCustomHeight(t *testing.T, height int64) (*app.BabylonApp, sdk.Context) {
	babylonApp := app.Setup(t, false)
	ctx := babylonApp.BaseApp.NewContext(false).
		WithBlockHeader(cmtproto.Header{Height: height, Time: time.Now().UTC()})
	return babylonApp, ctx
}

func FundAccount(
	t *testing.T,
	ctx sdk.Context,
	bbn *app.BabylonApp,
	acc sdk.AccAddress,
) {
	err := MintCoinsTo(bbn.BankKeeper, ctx, acc, sdk.NewCoins(
		sdk.NewCoin("ubbn", math.NewInt(10000000000)),
	))
	require.NoError(t, err)
}

func MintCoinsTo(
	bankKeeper bankkeeper.Keeper,
	ctx sdk.Context,
	addr sdk.AccAddress,
	amounts sdk.Coins) error {
	if err := bankKeeper.MintCoins(ctx, minttypes.ModuleName, amounts); err != nil {
		return err
	}

	return bankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, amounts)
}

func StoreTestCode(
	t *testing.T,
	ctx sdk.Context,
	babylonApp *app.BabylonApp,
	addr sdk.AccAddress,
	codePath string,
) (uint64, []byte) {
	codePath = filepath.Clean(codePath)
	wasmCode, err := os.ReadFile(codePath)

	require.NoError(t, err)
	permKeeper := keeper.NewPermissionedKeeper(babylonApp.WasmKeeper, keeper.DefaultAuthorizationPolicy{})
	id, checksum, err := permKeeper.Create(ctx, addr, wasmCode, nil)
	require.NoError(t, err)
	return id, checksum
}

func DeployTestContract(
	t *testing.T,
	ctx sdk.Context,
	bbn *app.BabylonApp,
	deployer sdk.AccAddress,
	codePath string,
) sdk.AccAddress {
	codeId, _ := StoreTestCode(t, ctx, bbn, deployer, codePath)
	contractAddr := InstantiateContract(t, ctx, bbn, deployer, codeId)
	return contractAddr
}

func InstantiateContract(
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
