package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	cmtcrypto "github.com/cometbft/cometbft/crypto"
	"github.com/otiai10/copy"

	"cosmossdk.io/log"
	"github.com/babylonlabs-io/babylon/app"
	babylonApp "github.com/babylonlabs-io/babylon/app"
	appkeepers "github.com/babylonlabs-io/babylon/app/keepers"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	dbmc "github.com/cometbft/cometbft-db"
	cs "github.com/cometbft/cometbft/consensus"
	cometlog "github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/mempool"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/proxy"
	sm "github.com/cometbft/cometbft/state"
	"github.com/cometbft/cometbft/store"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	gogoprotoio "github.com/cosmos/gogoproto/io"
	"github.com/stretchr/testify/require"
)

var validatorConfig = &initialization.NodeConfig{
	Name:               "initValidator",
	Pruning:            "default",
	PruningKeepRecent:  "0",
	PruningInterval:    "0",
	SnapshotInterval:   1500,
	SnapshotKeepRecent: 2,
	IsValidator:        true,
}

const chainID = initialization.ChainAID

const testPartSize = 65536

func getGenDoc(
	t *testing.T, nodeDir string) (map[string]json.RawMessage, *genutiltypes.AppGenesis) {
	path := filepath.Join(nodeDir, "config", "genesis.json")
	fmt.Printf("path to gendoc: %s\n", path)

	genState, appGenesis, err := genutiltypes.GenesisStateFromGenFile(path)
	require.NoError(t, err)
	return genState, appGenesis
}

type AppOptionsMap map[string]interface{}

func (m AppOptionsMap) Get(key string) interface{} {
	v, ok := m[key]
	if !ok {
		return interface{}(nil)
	}

	return v
}

func NewAppOptionsWithFlagHome(homePath string) servertypes.AppOptions {
	return AppOptionsMap{
		flags.FlagHome:       homePath,
		"btc-config.network": "simnet",
		"pruning":            "nothing",
		"chain-id":           chainID,
		"app-db-backend":     "memdb",
	}
}

func getBlockId(t *testing.T, block *cmttypes.Block) cmttypes.BlockID {
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)
	return cmttypes.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}
}

type FinalizedBlock struct {
	Height uint64
	ID     cmttypes.BlockID
	Block  *cmttypes.Block
}

type BabylonAppDriver struct {
	App        *app.BabylonApp
	PrivSigner *appkeepers.PrivSigner
	BlockExec  *sm.BlockExecutor
	BlockStore *store.BlockStore
	StateStore *sm.Store
	NodeDir    string

	ValidatorAddress []byte

	FinalizedBlocks []FinalizedBlock
	LastState       sm.State
}

// Inititializes Babylon driver for block creation
// TODO: Add option to send txs to app
func NewBabylonAppDriver(
	t *testing.T,
	dir string,
	copyDir string,
) *BabylonAppDriver {
	chain, err := initialization.InitChain(
		chainID,
		dir,
		[]*initialization.NodeConfig{validatorConfig},
		3*time.Minute,
		1*time.Minute,
		1,
		[]*btclighttypes.BTCHeaderInfo{},
	)
	require.NoError(t, err)
	require.NotNil(t, chain)

	_, doc := getGenDoc(t, chain.Nodes[0].ConfigDir)
	fmt.Printf("config dir is path %s\n", chain.Nodes[0].ConfigDir)

	if copyDir != "" {
		// Copy dir is needed as otherwise
		err := copy.Copy(chain.Nodes[0].ConfigDir, copyDir)
		fmt.Printf("copying %s to %s\n", chain.Nodes[0].ConfigDir, copyDir)

		require.NoError(t, err)
	}

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

	signer, err := appkeepers.InitPrivSigner(chain.Nodes[0].ConfigDir)
	require.NoError(t, err)
	require.NotNil(t, signer)
	signerValAddress := signer.WrappedPV.GetAddress()
	fmt.Printf("signer val address: %s\n", signerValAddress.String())

	appOptions := NewAppOptionsWithFlagHome(chain.Nodes[0].ConfigDir)
	baseAppOptions := server.DefaultBaseappOptions(appOptions)
	tmpApp := babylonApp.NewBabylonApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		map[int64]bool{},
		0,
		signer,
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
	validatorAddress, _ := state.Validators.GetByIndex(0)

	return &BabylonAppDriver{
		App:              tmpApp,
		PrivSigner:       signer,
		BlockExec:        blockExec,
		BlockStore:       blockStore,
		StateStore:       &stateStore,
		NodeDir:          chain.Nodes[0].ConfigDir,
		ValidatorAddress: validatorAddress,
		FinalizedBlocks:  []FinalizedBlock{},
		LastState:        state.Copy(),
	}
}

func (d *BabylonAppDriver) GetLastFinalizedBlock() *FinalizedBlock {
	if len(d.FinalizedBlocks) == 0 {
		return nil
	}

	return &d.FinalizedBlocks[len(d.FinalizedBlocks)-1]
}

func signVoteExtension(
	t *testing.T,
	veBytes []byte,
	height uint64,
	valPrivKey cmtcrypto.PrivKey,
) []byte {
	cve := cmtproto.CanonicalVoteExtension{
		Extension: veBytes,
		Height:    int64(height),
		Round:     int64(0),
		ChainId:   chainID,
	}

	var cveBuffer bytes.Buffer
	err := gogoprotoio.NewDelimitedWriter(&cveBuffer).WriteMsg(&cve)
	require.NoError(t, err)
	cveBytes := cveBuffer.Bytes()
	extensionSig, err := valPrivKey.Sign(cveBytes)
	require.NoError(t, err)

	return extensionSig
}

func (d *BabylonAppDriver) GenerateNewBlock(t *testing.T) {
	if len(d.FinalizedBlocks) == 0 {
		extCommitFirsBlock := &cmttypes.ExtendedCommit{}
		block1, err := d.BlockExec.CreateProposalBlock(
			context.Background(),
			1,
			d.LastState,
			extCommitFirsBlock,
			d.ValidatorAddress,
		)
		require.NoError(t, err)
		require.NotNil(t, block1)

		accepted, err := d.BlockExec.ProcessProposal(block1, d.LastState)
		require.NoError(t, err)
		require.True(t, accepted)

		block1ID := getBlockId(t, block1)
		state, err := d.BlockExec.ApplyVerifiedBlock(d.LastState, block1ID, block1)
		require.NoError(t, err)
		require.NotNil(t, state)

		d.FinalizedBlocks = append(d.FinalizedBlocks, FinalizedBlock{
			Height: 1,
			ID:     block1ID,
			Block:  block1,
		})
		d.LastState = state.Copy()
	} else {
		lastFinalizedBlock := d.GetLastFinalizedBlock()

		var extension []byte

		if lastFinalizedBlock.Height > 1 {
			ext, err := d.BlockExec.ExtendVote(
				context.Background(),
				&cmttypes.Vote{
					BlockID: lastFinalizedBlock.ID,
					Height:  int64(lastFinalizedBlock.Height),
				},
				lastFinalizedBlock.Block,
				d.LastState,
			)
			require.NoError(t, err)
			extension = ext
		} else {
			extension = []byte{}
		}

		extensionSig := signVoteExtension(
			t,
			extension,
			lastFinalizedBlock.Height,
			d.PrivSigner.WrappedPV.GetValPrivKey(),
		)

		// We are adding invalid signatures here as we are not validating them in
		// ApplyBlock
		extCommitSig := cmttypes.ExtendedCommitSig{
			CommitSig: cmttypes.CommitSig{
				BlockIDFlag:      cmttypes.BlockIDFlagCommit,
				ValidatorAddress: d.ValidatorAddress,
				Timestamp:        time.Now().Add(1 * time.Second),
				Signature:        []byte("test"),
			},
			Extension:          extension,
			ExtensionSignature: extensionSig,
		}

		oneValExtendedCommit := &cmttypes.ExtendedCommit{
			Height:  int64(lastFinalizedBlock.Height),
			Round:   0,
			BlockID: lastFinalizedBlock.ID,
			ExtendedSignatures: []cmttypes.ExtendedCommitSig{
				extCommitSig,
			},
		}

		block1, err := d.BlockExec.CreateProposalBlock(
			context.Background(),
			int64(lastFinalizedBlock.Height)+1,
			d.LastState,
			oneValExtendedCommit,
			d.ValidatorAddress,
		)
		require.NoError(t, err)
		require.NotNil(t, block1)

		// it is here as it is good sanity check for all babylon custom validations
		accepted, err := d.BlockExec.ProcessProposal(block1, d.LastState)
		require.NoError(t, err)
		require.True(t, accepted)

		block1ID := getBlockId(t, block1)
		state, err := d.BlockExec.ApplyVerifiedBlock(d.LastState, block1ID, block1)
		require.NoError(t, err)
		require.NotNil(t, state)

		d.FinalizedBlocks = append(d.FinalizedBlocks, FinalizedBlock{
			Height: lastFinalizedBlock.Height + 1,
			ID:     block1ID,
			Block:  block1,
		})
		d.LastState = state.Copy()
	}
}

type BlockReplayer struct {
	BlockExec *sm.BlockExecutor
	LastState sm.State
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

	signer, err := appkeepers.InitPrivSigner(nodeDir)
	require.NoError(t, err)
	require.NotNil(t, signer)
	signerValAddress := signer.WrappedPV.GetAddress()
	fmt.Printf("signer val address: %s\n", signerValAddress.String())

	appOptions := NewAppOptionsWithFlagHome(nodeDir)
	baseAppOptions := server.DefaultBaseappOptions(appOptions)
	tmpApp := babylonApp.NewBabylonApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		map[int64]bool{},
		0,
		signer,
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
	}
}

func (r *BlockReplayer) ReplayBlocks(t *testing.T, blocks []FinalizedBlock) {
	for _, block := range blocks {
		blockID := getBlockId(t, block.Block)
		state, err := r.BlockExec.ApplyVerifiedBlock(r.LastState, blockID, block.Block)
		require.NoError(t, err)
		require.NotNil(t, state)
		r.LastState = state.Copy()
	}
}

func TestReplayBlocks(t *testing.T) {
	driverTempDir, err := os.MkdirTemp("", "test-app-event")
	require.NoError(t, err)

	replayerTempDir, err := os.MkdirTemp("", "test-app-event")
	require.NoError(t, err)

	defer os.RemoveAll(driverTempDir)
	defer os.RemoveAll(replayerTempDir)
	driver := NewBabylonAppDriver(t, driverTempDir, replayerTempDir)

	for i := 0; i < 100; i++ {
		driver.GenerateNewBlock(t)
	}

	replayer := NewBlockReplayer(t, replayerTempDir)

	replayer.ReplayBlocks(t, driver.FinalizedBlocks)

	// after replay we should have the same apphash
	require.Equal(t, driver.LastState.LastBlockHeight, replayer.LastState.LastBlockHeight)
	require.Equal(t, driver.LastState.AppHash, replayer.LastState.AppHash)
}
