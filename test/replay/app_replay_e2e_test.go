package replay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	goMath "math"
	"math/rand"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/app"
	babylonApp "github.com/babylonlabs-io/babylon/app"
	appkeepers "github.com/babylonlabs-io/babylon/app/keepers"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	dbmc "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	cs "github.com/cometbft/cometbft/consensus"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cometlog "github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/mempool"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/proxy"
	sm "github.com/cometbft/cometbft/state"
	"github.com/cometbft/cometbft/store"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	gogoprotoio "github.com/cosmos/gogoproto/io"
	"github.com/otiai10/copy"
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

const (
	chainID      = initialization.ChainAID
	testPartSize = 65536

	defaultGasLimit = 500000
	defaultFee      = 500000
	epochLength     = 10
)

var (
	defaultFeeCoin                 = sdk.NewCoin("ubbn", math.NewInt(defaultFee))
	BtcParams                      = &chaincfg.SimNetParams
	covenantSKs, _, CovenantQuorum = bstypes.DefaultCovenantCommittee()
)

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
	App                  *app.BabylonApp
	PrivSigner           *appkeepers.PrivSigner
	DriverAccountPrivKey cryptotypes.PrivKey
	DriverAccountSeqNr   uint64
	DriverAccountAccNr   uint64
	BlockExec            *sm.BlockExecutor
	BlockStore           *store.BlockStore
	StateStore           sm.Store
	NodeDir              string
	ValidatorAddress     []byte
	FinalizedBlocks      []FinalizedBlock
	LastState            sm.State
}

// Inititializes Babylon driver for block creation
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

	validatorPrivKey := secp256k1.PrivKey{
		Key: chain.Nodes[0].PrivateKey,
	}

	return &BabylonAppDriver{
		App:                  tmpApp,
		PrivSigner:           signer,
		DriverAccountPrivKey: &validatorPrivKey,
		// Driver account always start from 1, as we executed tx for creating validator
		// in genesis block
		DriverAccountSeqNr: 1,
		DriverAccountAccNr: 0,
		BlockExec:          blockExec,
		BlockStore:         blockStore,
		StateStore:         stateStore,
		NodeDir:            chain.Nodes[0].ConfigDir,
		ValidatorAddress:   validatorAddress,
		FinalizedBlocks:    []FinalizedBlock{},
		LastState:          state.Copy(),
	}
}

func (d *BabylonAppDriver) GetLastFinalizedBlock() *FinalizedBlock {
	if len(d.FinalizedBlocks) == 0 {
		return nil
	}

	return &d.FinalizedBlocks[len(d.FinalizedBlocks)-1]
}

func (d *BabylonAppDriver) GetContextForLastFinalizedBlock() sdk.Context {
	lastFinalizedBlock := d.GetLastFinalizedBlock()
	return d.App.NewUncachedContext(false, *lastFinalizedBlock.Block.Header.ToProto())
}

type senderInfo struct {
	privKey        cryptotypes.PrivKey
	sequenceNumber uint64
	accountNumber  uint64
}

func createTx(
	t *testing.T,
	txConfig client.TxConfig,
	senderInfo *senderInfo,
	gas uint64,
	fee sdk.Coin,
	msgs ...sdk.Msg,
) []byte {
	txBuilder := txConfig.NewTxBuilder()
	txBuilder.SetGasLimit(gas)
	txBuilder.SetFeeAmount(sdk.NewCoins(fee))
	txBuilder.SetMsgs(msgs...)

	sigV2 := signing.SignatureV2{
		PubKey: senderInfo.privKey.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode(txConfig.SignModeHandler().DefaultMode()),
			Signature: nil,
		},
		Sequence: senderInfo.sequenceNumber,
	}

	err := txBuilder.SetSignatures(sigV2)
	require.NoError(t, err)

	signerData := xauthsigning.SignerData{
		ChainID:       chainID,
		AccountNumber: senderInfo.accountNumber,
		Sequence:      senderInfo.sequenceNumber,
	}

	sigV2, err = tx.SignWithPrivKey(
		context.Background(),
		signing.SignMode(txConfig.SignModeHandler().DefaultMode()),
		signerData,
		txBuilder,
		senderInfo.privKey,
		txConfig,
		senderInfo.sequenceNumber,
	)
	require.NoError(t, err)

	err = txBuilder.SetSignatures(sigV2)
	require.NoError(t, err)

	txBytes, err := txConfig.TxEncoder()(txBuilder.GetTx())
	require.NoError(t, err)

	return txBytes
}

func (d *BabylonAppDriver) CreateTx(
	t *testing.T,
	senderInfo *senderInfo,
	gas uint64,
	fee sdk.Coin,
	msgs ...sdk.Msg,
) []byte {
	return createTx(t, d.App.TxConfig(), senderInfo, gas, fee, msgs...)
}

// SendTxWithMessagesSuccess sends tx with msgs to the mempool and asserts that
// execution was successful
func (d *BabylonAppDriver) SendTxWithMessagesSuccess(
	t *testing.T,
	senderInfo *senderInfo,
	gas uint64,
	fee sdk.Coin,
	msgs ...sdk.Msg,
) {
	txBytes := d.CreateTx(t, senderInfo, gas, fee, msgs...)

	result, err := d.App.CheckTx(&abci.RequestCheckTx{
		Tx:   txBytes,
		Type: abci.CheckTxType_New,
	})
	require.NoError(t, err)
	require.Equal(t, result.Code, uint32(0))
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

func (d *BabylonAppDriver) GenerateNewBlock(t *testing.T) *abci.ResponseFinalizeBlock {
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

		lastResponse, err := d.StateStore.LoadFinalizeBlockResponse(1)
		require.NoError(t, err)
		require.NotNil(t, lastResponse)
		return lastResponse
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

		lastResponse, err := d.StateStore.LoadFinalizeBlockResponse(state.LastBlockHeight)
		require.NoError(t, err)
		require.NotNil(t, lastResponse)
		return lastResponse
	}
}

func (d *BabylonAppDriver) GenerateNewBlockAssertExecutionSuccess(
	t *testing.T,
	expectedTxNumber int,
) {
	response := d.GenerateNewBlock(t)

	require.Equal(t, len(response.TxResults), expectedTxNumber)
	for _, tx := range response.TxResults {
		require.Equal(t, tx.Code, uint32(0))
	}
}

func (d *BabylonAppDriver) GetDriverAccountAddress() sdk.AccAddress {
	return sdk.AccAddress(d.DriverAccountPrivKey.PubKey().Address())
}

func (d *BabylonAppDriver) GetDriverAccountSenderInfo() *senderInfo {
	return &senderInfo{
		privKey:        d.DriverAccountPrivKey,
		sequenceNumber: d.DriverAccountSeqNr,
		accountNumber:  d.DriverAccountAccNr,
	}
}

func (d *BabylonAppDriver) GetBTCLCTip() (*wire.BlockHeader, uint32) {
	tipInfo := d.App.BTCLightClientKeeper.GetTipInfo(d.GetContextForLastFinalizedBlock())
	return tipInfo.Header.ToBlockHeader(), tipInfo.Height
}

func (d *BabylonAppDriver) GetAllBTCDelegations(t *testing.T) []*bstypes.BTCDelegationResponse {
	pagination := &query.PageRequest{}
	pagination.Limit = goMath.MaxUint32

	delegations, err := d.App.BTCStakingKeeper.BTCDelegations(d.GetContextForLastFinalizedBlock(), &bstypes.QueryBTCDelegationsRequest{
		Status:     bstypes.BTCDelegationStatus_ANY,
		Pagination: pagination,
	})
	require.NoError(t, err)
	return delegations.BtcDelegations
}

// SendTxWithMsgsFromDriverAccount sends tx with msgs from driver account and asserts that
// SendTxWithMsgsFromDriverAccount sends tx with msgs from driver account and asserts that
// execution was successful. It assumes that there will only be one tx in the block.
func (d *BabylonAppDriver) SendTxWithMsgsFromDriverAccount(
	t *testing.T,
	msgs ...sdk.Msg) {
	d.SendTxWithMessagesSuccess(
		t,
		d.GetDriverAccountSenderInfo(),
		defaultGasLimit,
		defaultFeeCoin,
		msgs...,
	)
	d.GenerateNewBlockAssertExecutionSuccess(t, 1)

	d.DriverAccountSeqNr++
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
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
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

func TestSendingTxFromDriverAccount(t *testing.T) {
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(t, driverTempDir, replayerTempDir)

	// go over epoch boundary
	for i := 0; i < 1+epochLength; i++ {
		driver.GenerateNewBlock(t)
	}

	_, _, addr1 := testdata.KeyTestPubAddr()
	toAddr := addr1.String()

	transferMsg := &banktypes.MsgSend{
		FromAddress: driver.GetDriverAccountAddress().String(),
		ToAddress:   toAddr,
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("ubbn", 10000)),
	}

	driver.SendTxWithMsgsFromDriverAccount(t, transferMsg)
	driver.SendTxWithMsgsFromDriverAccount(t, transferMsg)
	driver.SendTxWithMsgsFromDriverAccount(t, transferMsg)
	driver.SendTxWithMsgsFromDriverAccount(t, transferMsg)

	// check that replayer has the same state as driver, as we replayed all blocks
	replayer := NewBlockReplayer(t, replayerTempDir)
	replayer.ReplayBlocks(t, driver.FinalizedBlocks)

	// after replay we should have the same apphash
	require.Equal(t, driver.LastState.LastBlockHeight, replayer.LastState.LastBlockHeight)
	require.Equal(t, driver.LastState.AppHash, replayer.LastState.AppHash)
}

func blocksToHeaderBytes(blocks []*wire.MsgBlock) []bbn.BTCHeaderBytes {
	headerBytes := []bbn.BTCHeaderBytes{}
	for _, block := range blocks {
		headerBytes = append(headerBytes, bbn.NewBTCHeaderBytesFromBlockHeader(&block.Header))
	}
	return headerBytes
}

func TestFoo(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(t, driverTempDir, replayerTempDir)

	// go over epoch boundary
	for i := 0; i < 1+epochLength; i++ {
		driver.GenerateNewBlock(t)
	}

	tip, tipHeight := driver.GetBTCLCTip()
	numbBlocks := uint64(10)
	blocks := datagen.GenNEmptyBlocks(r, numbBlocks, tip)
	headers := blocksToHeaderBytes(blocks)

	driver.SendTxWithMsgsFromDriverAccount(t, &btclighttypes.MsgInsertHeaders{
		Signer:  driver.GetDriverAccountAddress().String(),
		Headers: headers,
	})

	_, newTipHeight := driver.GetBTCLCTip()
	require.Equal(t, newTipHeight, tipHeight+uint32(numbBlocks))

	// lastFinalizedBlock := driver.GetLastFinalizedBlock()
	params := driver.App.BTCStakingKeeper.GetParams(
		driver.GetContextForLastFinalizedBlock(),
	)
	fmt.Printf("params: %+v\n", params)

	prv, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	msg, err := datagen.GenRandomCreateFinalityProviderMsgWithBTCBabylonSKs(
		r,
		prv,
		driver.GetDriverAccountAddress(),
	)
	require.NoError(t, err)
	driver.SendTxWithMsgsFromDriverAccount(t, msg)

	stakerPrv, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	createDelegationMsg := datagen.GenRandomMsgCreateBtcDelegationAndMsgAddCovenantSignatures(
		r,
		t,
		BtcParams,
		driver.GetDriverAccountAddress(),
		[]bbn.BIP340PubKey{*msg.BtcPk},
		stakerPrv,
		covenantSKs,
		&params,
	)

	driver.SendTxWithMsgsFromDriverAccount(t, createDelegationMsg)
	delegations := driver.GetAllBTCDelegations(t)
	// There should be one delegation
	require.Equal(t, len(delegations), 1)
}
