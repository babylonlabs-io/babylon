package replay

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	goMath "math"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/btctxformatter"
	bbn "github.com/babylonlabs-io/babylon/types"
	btckckpttypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	ckpttypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	et "github.com/babylonlabs-io/babylon/x/epoching/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	dbmc "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	cs "github.com/cometbft/cometbft/consensus"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/ed25519"
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
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	gogoprotoio "github.com/cosmos/gogoproto/io"
	"github.com/otiai10/copy"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	babylonApp "github.com/babylonlabs-io/babylon/app"
	appsigner "github.com/babylonlabs-io/babylon/app/signer"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
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

	defaultGasLimit = 5000000
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

func MsgsToSdkMsg[M sdk.Msg](msgs []M) []sdk.Msg {
	sdkMsgs := make([]sdk.Msg, len(msgs))
	for i, msg := range msgs {
		sdkMsgs[i] = msg
	}
	return sdkMsgs
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
	BlsSigner            checkpointingtypes.BlsSigner
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
	DelegatorAddress     sdk.ValAddress
	CometPrivKey         cmtcrypto.PrivKey
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

	blsSigner, err := appsigner.InitBlsSigner(chain.Nodes[0].ConfigDir)
	require.NoError(t, err)
	require.NotNil(t, blsSigner)
	signerValAddress := sdk.ValAddress(chain.Nodes[0].PublicAddress)
	require.NoError(t, err)
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
	validatorAddress, _ := state.Validators.GetByIndex(0)

	validatorPrivKey := secp256k1.PrivKey{
		Key: chain.Nodes[0].PrivateKey,
	}

	return &BabylonAppDriver{
		App:                  tmpApp,
		BlsSigner:            *blsSigner,
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
		DelegatorAddress:   signerValAddress,
		CometPrivKey:       ed25519.PrivKey(chain.Nodes[0].CometPrivKey),
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
	err := txBuilder.SetMsgs(msgs...)
	require.NoError(t, err)

	sigV2 := signing.SignatureV2{
		PubKey: senderInfo.privKey.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode(txConfig.SignModeHandler().DefaultMode()),
			Signature: nil,
		},
		Sequence: senderInfo.sequenceNumber,
	}

	err = txBuilder.SetSignatures(sigV2)
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
			d.CometPrivKey,
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
) {
	response := d.GenerateNewBlock(t)

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

func blocksWithProofsToHeaderBytes(blocks []*datagen.BlockWithProofs) []bbn.BTCHeaderBytes {
	headerBytes := []bbn.BTCHeaderBytes{}
	for _, block := range blocks {
		headerBytes = append(headerBytes, bbn.NewBTCHeaderBytesFromBlockHeader(&block.Block.Header))
	}
	return headerBytes
}

func (d *BabylonAppDriver) ExtendBTCLcWithNEmptyBlocks(
	r *rand.Rand,
	t *testing.T,
	n uint32,
) (*wire.BlockHeader, uint32) {
	tip, _ := d.GetBTCLCTip()
	blocks := datagen.GenNEmptyBlocks(r, uint64(n), tip)
	headers := blocksWithProofsToHeaderBytes(blocks)

	d.SendTxWithMsgsFromDriverAccount(t, &btclighttypes.MsgInsertHeaders{
		Signer:  d.GetDriverAccountAddress().String(),
		Headers: headers,
	})

	newTip, newTipHeight := d.GetBTCLCTip()
	return newTip, newTipHeight
}

func (d *BabylonAppDriver) GenBlockWithTransactions(
	r *rand.Rand,
	t *testing.T,
	txs []*wire.MsgTx,
) *datagen.BlockWithProofs {
	tip, _ := d.GetBTCLCTip()
	block := datagen.GenRandomBtcdBlockWithTransactions(r, txs, tip)
	headers := blocksWithProofsToHeaderBytes([]*datagen.BlockWithProofs{block})

	d.SendTxWithMsgsFromDriverAccount(t, &btclighttypes.MsgInsertHeaders{
		Signer:  d.GetDriverAccountAddress().String(),
		Headers: headers,
	})

	return block
}

func (d *BabylonAppDriver) getDelegationWithStatus(t *testing.T, status bstypes.BTCDelegationStatus) []*bstypes.BTCDelegationResponse {
	pagination := &query.PageRequest{}
	pagination.Limit = goMath.MaxUint32

	delegations, err := d.App.BTCStakingKeeper.BTCDelegations(d.GetContextForLastFinalizedBlock(), &bstypes.QueryBTCDelegationsRequest{
		Status:     status,
		Pagination: pagination,
	})
	require.NoError(t, err)
	return delegations.BtcDelegations
}

func (d *BabylonAppDriver) GetAllBTCDelegations(t *testing.T) []*bstypes.BTCDelegationResponse {
	return d.getDelegationWithStatus(t, bstypes.BTCDelegationStatus_ANY)
}

func (d *BabylonAppDriver) GetVerifiedBTCDelegations(t *testing.T) []*bstypes.BTCDelegationResponse {
	return d.getDelegationWithStatus(t, bstypes.BTCDelegationStatus_VERIFIED)
}

func (d *BabylonAppDriver) GetActiveBTCDelegations(t *testing.T) []*bstypes.BTCDelegationResponse {
	return d.getDelegationWithStatus(t, bstypes.BTCDelegationStatus_ACTIVE)
}

func (d *BabylonAppDriver) GetBTCStakingParams(t *testing.T) *bstypes.Params {
	params := d.App.BTCStakingKeeper.GetParams(d.GetContextForLastFinalizedBlock())
	return &params
}

func (d *BabylonAppDriver) GetEpochingParams() et.Params {
	return d.App.EpochingKeeper.GetParams(d.GetContextForLastFinalizedBlock())
}

func (d *BabylonAppDriver) GetEpoch() *et.Epoch {
	return d.App.EpochingKeeper.GetEpoch(d.GetContextForLastFinalizedBlock())
}

func (d *BabylonAppDriver) GetCheckpoint(
	t *testing.T,
	epochNumber uint64,
) *ckpttypes.RawCheckpointWithMeta {
	checkpoint, err := d.App.CheckpointingKeeper.GetRawCheckpoint(d.GetContextForLastFinalizedBlock(), epochNumber)
	require.NoError(t, err)
	return checkpoint
}

func (d *BabylonAppDriver) GetLastFinalizedEpoch() uint64 {
	return d.App.CheckpointingKeeper.GetLastFinalizedEpoch(d.GetContextForLastFinalizedBlock())
}

func (d *BabylonAppDriver) GenCkptForEpoch(r *rand.Rand, t *testing.T, epochNumber uint64) {
	ckptWithMeta := d.GetCheckpoint(t, epochNumber)
	subAddress := d.GetDriverAccountAddress()
	subAddressBytes := subAddress.Bytes()

	rawCkpt, err := ckpttypes.FromRawCkptToBTCCkpt(ckptWithMeta.Ckpt, subAddressBytes)
	require.NoError(t, err)

	tagBytes, err := hex.DecodeString(initialization.BabylonOpReturnTag)
	require.NoError(t, err)

	data1, data2 := btctxformatter.MustEncodeCheckpointData(
		btctxformatter.BabylonTag(tagBytes),
		btctxformatter.CurrentVersion,
		rawCkpt,
	)

	tx1 := datagen.CreatOpReturnTransaction(r, data1)
	tx2 := datagen.CreatOpReturnTransaction(r, data2)

	blockWithProofs := d.GenBlockWithTransactions(r, t, []*wire.MsgTx{tx1, tx2})

	msg := btckckpttypes.MsgInsertBTCSpvProof{
		Submitter: subAddress.String(),
		Proofs:    blockWithProofs.Proofs[1:],
	}

	d.SendTxWithMsgsFromDriverAccount(t, &msg)
}

func (d *BabylonAppDriver) FinializeCkptForEpoch(r *rand.Rand, t *testing.T, epochNumber uint64) {
	lastFinalizedEpoch := d.GetLastFinalizedEpoch()
	require.Equal(t, lastFinalizedEpoch+1, epochNumber)

	btckptParams := d.GetBTCCkptParams(t)
	d.GenCkptForEpoch(r, t, epochNumber)

	_, _ = d.ExtendBTCLcWithNEmptyBlocks(r, t, btckptParams.CheckpointFinalizationTimeout)

	lastFinalizedEpoch = d.GetLastFinalizedEpoch()
	require.Equal(t, lastFinalizedEpoch, epochNumber)
}

func (d *BabylonAppDriver) GetBTCCkptParams(t *testing.T) btckckpttypes.Params {
	return d.App.BtcCheckpointKeeper.GetParams(d.GetContextForLastFinalizedBlock())
}

func (d *BabylonAppDriver) ProgressTillFirstBlockTheNextEpoch(t *testing.T) {
	currnetEpochNunber := d.GetEpoch().EpochNumber
	nextEpochNumber := currnetEpochNunber + 1

	for currnetEpochNunber < nextEpochNumber {
		d.GenerateNewBlock(t)
		currnetEpochNunber = d.GetEpoch().EpochNumber
	}
}

func (d *BabylonAppDriver) GetActiveFpsAtHeight(t *testing.T, height uint64) []*ftypes.ActiveFinalityProvidersAtHeightResponse {
	res, err := d.App.FinalityKeeper.ActiveFinalityProvidersAtHeight(
		d.GetContextForLastFinalizedBlock(),
		&ftypes.QueryActiveFinalityProvidersAtHeightRequest{
			Height:     height,
			Pagination: &query.PageRequest{},
		},
	)
	require.NoError(t, err)
	return res.FinalityProviders
}

func (d *BabylonAppDriver) GetActiveFpsAtCurrentHeight(t *testing.T) []*ftypes.ActiveFinalityProvidersAtHeightResponse {
	return d.GetActiveFpsAtHeight(t, d.GetLastFinalizedBlock().Height)
}

func (d *BabylonAppDriver) WaitTillAllFpsJailed(t *testing.T) {
	for {
		activeFps := d.GetActiveFpsAtCurrentHeight(t)
		if len(activeFps) == 0 {
			break
		}
		d.GenerateNewBlock(t)
	}
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

	result := d.GenerateNewBlock(t)

	for _, rs := range result.TxResults {
		// our checkpoint transactions have 0 gas wanted, skip them to avoid confusing the
		// tests
		if rs.GasWanted == 0 {
			continue
		}

		// all executions should be successful
		require.Equal(t, rs.Code, uint32(0))
	}

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

type FinalityProviderInfo struct {
	MsgCreateFinalityProvider *bstypes.MsgCreateFinalityProvider
	BTCPrivateKey             *btcec.PrivateKey
	BabylonAddress            sdk.AccAddress
}

func GenerateNFinalityProviders(
	r *rand.Rand,
	t *testing.T,
	n uint32,
	senderAddress sdk.AccAddress,
) []*FinalityProviderInfo {
	var infos []*FinalityProviderInfo
	for i := uint32(0); i < n; i++ {
		prv, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		msg, err := datagen.GenRandomCreateFinalityProviderMsgWithBTCBabylonSKs(
			r,
			prv,
			senderAddress,
		)
		require.NoError(t, err)

		infos = append(infos, &FinalityProviderInfo{
			MsgCreateFinalityProvider: msg,
			BTCPrivateKey:             prv,
			BabylonAddress:            senderAddress,
		})
	}

	return infos
}

func FpInfosToMsgs(fpInfos []*FinalityProviderInfo) []sdk.Msg {
	msgs := []sdk.Msg{}
	for _, fpInfo := range fpInfos {
		msgs = append(msgs, fpInfo.MsgCreateFinalityProvider)
	}
	return msgs
}

func GenerateNBTCDelegationsForFinalityProvider(
	r *rand.Rand,
	t *testing.T,
	n uint32,
	senderAddress sdk.AccAddress,
	fpInfo *FinalityProviderInfo,
	params *bstypes.Params,
) []*datagen.CreateDelegationInfo {
	var delInfos []*datagen.CreateDelegationInfo

	for i := uint32(0); i < n; i++ {
		// TODO this slow due the key generation
		stakerPrv, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		delInfo := datagen.GenRandomMsgCreateBtcDelegationAndMsgAddCovenantSignatures(
			r,
			t,
			BtcParams,
			senderAddress,
			[]bbn.BIP340PubKey{*fpInfo.MsgCreateFinalityProvider.BtcPk},
			stakerPrv,
			covenantSKs,
			params,
		)
		delInfos = append(delInfos, delInfo)
	}

	return delInfos
}

func ToCreateBTCDelegationMsgs(
	delInfos []*datagen.CreateDelegationInfo,
) []sdk.Msg {
	msgs := []sdk.Msg{}
	for _, delInfo := range delInfos {
		msgs = append(msgs, delInfo.MsgCreateBTCDelegation)
	}
	return msgs
}

func ToCovenantSignaturesMsgs(
	delInfos []*datagen.CreateDelegationInfo,
) []sdk.Msg {
	msgs := []sdk.Msg{}
	for _, delInfo := range delInfos {
		msgs = append(msgs, MsgsToSdkMsg(delInfo.MsgAddCovenantSigs)...)
	}
	return msgs
}

func DelegationInfosToBTCTx(
	delInfos []*datagen.CreateDelegationInfo,
) []*wire.MsgTx {
	txs := []*wire.MsgTx{}
	for _, delInfo := range delInfos {
		txs = append(txs, delInfo.StakingTx)
	}
	return txs
}
