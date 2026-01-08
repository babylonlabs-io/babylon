package keeper

import (
	"context"
	"testing"

	"cosmossdk.io/core/header"
	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	"github.com/btcsuite/btcd/wire"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	bapp "github.com/babylonlabs-io/babylon/v4/app"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btclightclientk "github.com/babylonlabs-io/babylon/v4/x/btclightclient/keeper"
	btclightclientt "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
)

type MockIncentiveKeeper struct{}

func (mik MockIncentiveKeeper) IndexRefundableMsg(ctx context.Context, msg sdk.Msg) {}

func BTCLightClientKeeper(t testing.TB) (*btclightclientk.Keeper, sdk.Context) {
	k, ctx, _ := BTCLightClientKeeperWithCustomParams(t, btclightclientt.DefaultParams())
	return k, ctx
}

// NewBTCHeaderBytesList takes a list of block headers and parses it to BTCHeaderBytes.
func NewBTCHeaderBytesList(chain []*wire.BlockHeader) []bbn.BTCHeaderBytes {
	chainBytes := make([]bbn.BTCHeaderBytes, len(chain))
	for i, header := range chain {
		chainBytes[i] = bbn.NewBTCHeaderBytesFromBlockHeader(header)
	}
	return chainBytes
}

func BTCLightClientKeeperWithCustomParams(
	t testing.TB,
	p btclightclientt.Params,
) (*btclightclientk.Keeper, sdk.Context, corestore.KVStoreService) {
	storeKey := storetypes.NewKVStoreKey(btclightclientt.StoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	appOpts, cleanup := bapp.TmpAppOptions()
	defer cleanup()
	testCfg := bbn.ParseBtcOptionsFromConfig(appOpts)
	stServ := runtime.NewKVStoreService(storeKey)
	k := btclightclientk.NewKeeper(
		cdc,
		stServ,
		testCfg,
		&MockIncentiveKeeper{},
		appparams.AccGov.String(),
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{}, false, log.NewNopLogger())
	ctx = ctx.WithHeaderInfo(header.Info{})

	if err := k.SetParams(ctx, p); err != nil {
		panic(err)
	}

	return &k, ctx, stServ
}

// ParseBTCHeaderInfoResponseToInfo turns an BTCHeaderInfoResponse back to BTCHeaderInfo.
func ParseBTCHeaderInfoResponseToInfo(r *btclightclientt.BTCHeaderInfoResponse) (*btclightclientt.BTCHeaderInfo, error) {
	header, err := bbn.NewBTCHeaderBytesFromHex(r.HeaderHex)
	if err != nil {
		return nil, err
	}

	hash, err := bbn.NewBTCHeaderHashBytesFromHex(r.HashHex)
	if err != nil {
		return nil, err
	}

	return &btclightclientt.BTCHeaderInfo{
		Header: &header,
		Hash:   &hash,
		Height: r.Height,
		Work:   &r.Work,
	}, nil
}
