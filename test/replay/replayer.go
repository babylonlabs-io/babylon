package replay

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"cosmossdk.io/log"
	babylonApp "github.com/babylonlabs-io/babylon/v4/app"
	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	dbmc "github.com/cometbft/cometbft-db"
	cs "github.com/cometbft/cometbft/consensus"
	cometlog "github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/mempool"
	"github.com/cometbft/cometbft/proxy"
	sm "github.com/cometbft/cometbft/state"
	"github.com/cometbft/cometbft/store"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/stretchr/testify/require"
)

type BlockReplayer struct {
	BlockExec *sm.BlockExecutor
	LastState sm.State
	App       *babylonApp.BabylonApp
	Ctx       sdk.Context
}

func NewBlockReplayer(t *testing.T, nodeDir string) *BlockReplayer {
	_, doc := getGenDoc(t, nodeDir)

	genDoc, err := doc.ToGenesisDoc()
	require.NoError(t, err)

	state, err := sm.MakeGenesisState(genDoc)
	require.NoError(t, err)

	stateStore := sm.NewStore(dbmc.NewMemDB(), sm.StoreOptions{
		DiscardABCIResponses: false,
	})

	if err := stateStore.Save(state); err != nil {
		panic(err)
	}

	blsSigner, err := appsigner.InitBlsSigner(nodeDir)
	require.NoError(t, err)
	require.NotNil(t, blsSigner)

	appOptions := NewAppOptionsWithFlagHome(nodeDir)
	baseAppOptions := server.DefaultBaseappOptions(appOptions)
	tmpApp := babylonApp.NewBabylonApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		map[int64]bool{},
		0,
		blsSigner,
		appOptions,
		babylonApp.EmptyWasmOpts,
		baseAppOptions...,
	)

	cmtApp := server.NewCometABCIWrapper(tmpApp)
	procxyCons := proxy.NewMultiAppConn(
		proxy.NewLocalClientCreator(cmtApp),
		proxy.NopMetrics(),
	)
	err = procxyCons.Start()
	require.NoError(t, err)

	blockStore := store.NewBlockStore(dbmc.NewMemDB())

	blockExec := sm.NewBlockExecutor(
		stateStore,
		cometlog.TestingLogger(),
		procxyCons.Consensus(),
		&mempool.NopMempool{},
		sm.EmptyEvidencePool{},
		blockStore,
	)
	require.NotNil(t, blockExec)

	hs := cs.NewHandshaker(
		stateStore,
		state,
		blockStore,
		genDoc,
	)

	require.NotNil(t, hs)
	hs.SetLogger(cometlog.TestingLogger())
	err = hs.Handshake(procxyCons)
	require.NoError(t, err)

	state, err = stateStore.Load()
	require.NoError(t, err)
	require.NotNil(t, state)

	return &BlockReplayer{
		BlockExec: blockExec,
		LastState: state,
		App:       tmpApp,
	}
}

func (r *BlockReplayer) ReplayBlocks(t *testing.T, blocks []FinalizedBlock) {
	for _, block := range blocks {
		blockID, _ := getBlockId(t, block.Block)
		state, err := r.BlockExec.ApplyVerifiedBlock(r.LastState, blockID, block.Block)
		require.NoError(t, err)
		require.NotNil(t, state)
		r.LastState = state.Copy()
		r.Ctx = r.App.NewUncachedContext(false, *block.Block.Header.ToProto())
	}
}
