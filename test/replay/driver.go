package replay

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"math/rand"
	"path/filepath"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/btctxformatter"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btckckpttypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	ckpttypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"

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
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	gogoprotoio "github.com/cosmos/gogoproto/io"
	"github.com/otiai10/copy"
	"github.com/stretchr/testify/require"

	babylonApp "github.com/babylonlabs-io/babylon/v4/app"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	btclighttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"
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
	defaultFee   = 500000
	epochLength  = 10
	blkTime      = time.Second * 5
)

var (
	DefaultGasLimit                  = uint64(1_000_000)
	defaultFeeCoin                   = sdk.NewCoin("ubbn", math.NewInt(defaultFee))
	BtcParams                        = &chaincfg.SimNetParams
	covenantSKs, pks, CovenantQuorum = bstypes.LargeDefaultCovenantCommittee()
)

func getGenDoc(
	t *testing.T, nodeDir string) (map[string]json.RawMessage, *genutiltypes.AppGenesis) {
	path := filepath.Join(nodeDir, "config", "genesis.json")
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

func getBlockId(t *testing.T, block *cmttypes.Block) (cmttypes.BlockID, *cmttypes.PartSet) {
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)
	return cmttypes.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}, bps
}

type FinalizedBlock struct {
	Height uint64
	ID     cmttypes.BlockID
	Block  *cmttypes.Block
}

type BabylonAppDriver struct {
	*SenderInfo
	r            *rand.Rand
	t            *testing.T
	App          *babylonApp.BabylonApp
	BlsSigner    ckpttypes.BlsSigner
	BlockExec    *sm.BlockExecutor
	BlockStore   *store.BlockStore
	StateStore   sm.Store
	NodeDir      string
	CometAddress []byte
	ValAddress   sdk.ValAddress
	CometPrivKey cmtcrypto.PrivKey
	CurrentTime  time.Time
}

// NewBabylonAppDriverTmpDir initializes Babylon driver for block creation with
// temporary directories
func NewBabylonAppDriverTmpDir(r *rand.Rand, t *testing.T) *BabylonAppDriver {
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	return NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
}

// Inititializes Babylon driver for block creation
func NewBabylonAppDriver(
	r *rand.Rand,
	t *testing.T,
	dir string,
	copyDir string,
) *BabylonAppDriver {
	expeditedVotingPeriod := blkTime + time.Second

	chain, err := initialization.InitChain(
		chainID,
		dir,
		[]*initialization.NodeConfig{validatorConfig},
		expeditedVotingPeriod*2, // voting period
		expeditedVotingPeriod,   // expedited
		1,
		[]*btclighttypes.BTCHeaderInfo{},
		// 300M is gas limit set for testnet and mainnet
		300_000_000,
		&initialization.StartingBtcStakingParams{
			CovenantCommittee: bbn.NewBIP340PKsFromBTCPKs(pks),
			CovenantQuorum:    CovenantQuorum,
		},
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

	valAccAddr := sdk.MustAccAddressFromBech32(chain.Nodes[0].PublicAddress)
	signerValAddress := sdk.ValAddress(valAccAddr)
	require.NoError(t, err)

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
		r:         r,
		t:         t,
		App:       tmpApp,
		BlsSigner: *blsSigner,
		SenderInfo: &SenderInfo{
			privKey:        &validatorPrivKey,
			sequenceNumber: 1,
			accountNumber:  0,
		},
		BlockExec:    blockExec,
		BlockStore:   blockStore,
		StateStore:   stateStore,
		NodeDir:      chain.Nodes[0].ConfigDir,
		CometAddress: validatorAddress,
		ValAddress:   signerValAddress,
		CometPrivKey: ed25519.PrivKey(chain.Nodes[0].CometPrivKey),
		// initiate time to current time
		CurrentTime: time.Now(),
	}
}

func (d *BabylonAppDriver) Ctx() sdk.Context {
	return d.GetContextForLastFinalizedBlock()
}

func (d *BabylonAppDriver) GetLastFinalizedBlock() *FinalizedBlock {
	finalizedBlocks := d.GetFinalizedBlocks()

	if len(finalizedBlocks) == 0 {
		return nil
	}

	return &finalizedBlocks[len(finalizedBlocks)-1]
}

func (d *BabylonAppDriver) GetContextForLastFinalizedBlock() sdk.Context {
	lastFinalizedBlock := d.GetLastFinalizedBlock()
	return d.App.NewUncachedContext(false, *lastFinalizedBlock.Block.Header.ToProto())
}

type SenderInfo struct {
	privKey        cryptotypes.PrivKey
	sequenceNumber uint64
	accountNumber  uint64
}

func (s *SenderInfo) IncSeq() {
	s.sequenceNumber++
}

func (s *SenderInfo) Address() sdk.AccAddress {
	return sdk.AccAddress(s.privKey.PubKey().Address())
}

func (s *SenderInfo) AddressString() string {
	return s.Address().String()
}

func createTx(
	t *testing.T,
	txConfig client.TxConfig,
	senderInfo *SenderInfo,
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
	senderInfo *SenderInfo,
	gas uint64,
	fee sdk.Coin,
	msgs ...sdk.Msg,
) []byte {
	return createTx(t, d.App.TxConfig(), senderInfo, gas, fee, msgs...)
}

func (d *BabylonAppDriver) GenerateNewBlockReturnResults() []*abci.ExecTxResult {
	response := d.GenerateNewBlock()
	return response.TxResults
}

// SendTxWithMessagesSuccess sends tx with msgs to the mempool and asserts that
// execution was successful
func (d *BabylonAppDriver) SendTxWithMessagesSuccess(
	t *testing.T,
	senderInfo *SenderInfo,
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

func SendTxWithMessagesSuccess(
	t *testing.T,
	app *babylonApp.BabylonApp,
	senderInfo *SenderInfo,
	gas uint64,
	fee sdk.Coin,
	msgs ...sdk.Msg,
) {
	txBytes := createTx(t, app.TxConfig(), senderInfo, gas, fee, msgs...)

	result, err := app.CheckTx(&abci.RequestCheckTx{
		Tx:   txBytes,
		Type: abci.CheckTxType_New,
	})

	require.NoError(t, err)
	require.Equal(t, result.Code, uint32(0), result.Log)
}

func SendTxWithMessages(
	t *testing.T,
	app *babylonApp.BabylonApp,
	senderInfo *SenderInfo,
	msgs ...sdk.Msg,
) (*abci.ResponseCheckTx, error) {
	txBytes := createTx(t, app.TxConfig(), senderInfo, DefaultGasLimit, defaultFeeCoin, msgs...)

	return app.CheckTx(&abci.RequestCheckTx{
		Tx:   txBytes,
		Type: abci.CheckTxType_New,
	})
}

func DefaultSendTxWithMessagesSuccess(
	t *testing.T,
	app *babylonApp.BabylonApp,
	senderInfo *SenderInfo,
	msgs ...sdk.Msg,
) {
	SendTxWithMessagesSuccess(
		t,
		app,
		senderInfo,
		DefaultGasLimit,
		defaultFeeCoin,
		msgs...,
	)
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

func (d *BabylonAppDriver) GenerateNewBlock() *abci.ResponseFinalizeBlock {
	lastState, err := d.StateStore.Load()
	require.NoError(d.t, err)
	require.NotNil(d.t, lastState)

	var lastCommit *cmttypes.ExtendedCommit
	if lastState.LastBlockHeight == 0 {
		lastCommit = &cmttypes.ExtendedCommit{}
	} else {
		lastCommit = d.BlockStore.LoadBlockExtendedCommit(lastState.LastBlockHeight)
		require.NotNil(d.t, lastCommit)
	}

	block1, err := d.BlockExec.CreateProposalBlock(
		context.Background(),
		lastState.LastBlockHeight+1,
		lastState,
		lastCommit,
		d.CometAddress,
	)
	require.NoError(d.t, err)
	require.NotNil(d.t, block1)

	block1ID, partSet := getBlockId(d.t, block1)

	extension, err := d.BlockExec.ExtendVote(
		context.Background(),
		&cmttypes.Vote{
			BlockID: block1ID,
			Height:  block1.Height,
		},
		block1,
		lastState,
	)
	require.NoError(d.t, err)

	extensionSig := signVoteExtension(
		d.t,
		extension,
		uint64(block1.Height),
		d.CometPrivKey,
	)

	// We are adding invalid signatures here as we are not validating them in
	// ApplyBlock
	// add slepp to avoid zero duration for minting
	// Simulate 5s block time
	newTime := d.CurrentTime.Add(blkTime)
	extCommitSig := cmttypes.ExtendedCommitSig{
		CommitSig: cmttypes.CommitSig{
			BlockIDFlag:      cmttypes.BlockIDFlagCommit,
			ValidatorAddress: d.CometAddress,
			Timestamp:        newTime,
			Signature:        []byte("test"),
		},
		Extension:          extension,
		ExtensionSignature: extensionSig,
	}
	d.CurrentTime = newTime

	// Get current validator set to build proper commit
	validators := lastState.Validators
	extendedSignatures := make([]cmttypes.ExtendedCommitSig, len(validators.Validators))

	// Build commit signatures for each validator
	for i, val := range validators.Validators {
		if bytes.Equal(val.Address.Bytes(), d.CometAddress) {
			// This is our signing validator
			extendedSignatures[i] = extCommitSig
		} else {
			// Other validators are absent (not voting)
			extendedSignatures[i] = cmttypes.ExtendedCommitSig{
				CommitSig: cmttypes.CommitSig{
					BlockIDFlag: cmttypes.BlockIDFlagAbsent,
				},
			}
		}
	}

	oneValExtendedCommit := &cmttypes.ExtendedCommit{
		Height:             block1.Height,
		Round:              0,
		BlockID:            block1ID,
		ExtendedSignatures: extendedSignatures,
	}

	accepted, err := d.BlockExec.ProcessProposal(block1, lastState)
	require.NoError(d.t, err)
	require.True(d.t, accepted)

	state, err := d.BlockExec.ApplyVerifiedBlock(lastState, block1ID, block1)
	require.NoError(d.t, err)
	require.NotNil(d.t, state)

	d.BlockStore.SaveBlockWithExtendedCommit(
		block1,
		partSet,
		oneValExtendedCommit,
	)

	lastResponse, err := d.StateStore.LoadFinalizeBlockResponse(block1.Height)
	require.NoError(d.t, err)
	require.NotNil(d.t, lastResponse)
	return lastResponse
}

func (d *BabylonAppDriver) GetFinalizedBlocks() []FinalizedBlock {
	lastState, err := d.StateStore.Load()
	require.NoError(d.t, err)
	require.NotNil(d.t, lastState)

	blocks := []FinalizedBlock{}

	for i := int64(1); i <= lastState.LastBlockHeight; i++ {
		block := d.BlockStore.LoadBlock(i)
		require.NotNil(d.t, block)

		id, _ := getBlockId(d.t, block)

		blocks = append(blocks, FinalizedBlock{
			Height: uint64(block.Height),
			ID:     id,
			Block:  block,
		})
	}

	return blocks
}

func (d *BabylonAppDriver) GetLastState() sm.State {
	lastState, err := d.StateStore.Load()
	require.NoError(d.t, err)
	require.NotNil(d.t, lastState)
	return lastState
}

func (d *BabylonAppDriver) GenerateBlocksUntilHeight(untilBlock uint64) {
	blkHeight := d.Ctx().BlockHeader().Height
	for i := blkHeight; i < int64(untilBlock); i++ {
		d.GenerateNewBlockAssertExecutionSuccess()
	}
}

func (d *BabylonAppDriver) GenerateNewBlockAssertExecutionSuccess() {
	d.GenerateNewBlockAssertExecutionSuccessWithResults()
}

func (d *BabylonAppDriver) GenerateNewBlockAssertExecutionFailure() []*abci.ExecTxResult {
	response := d.GenerateNewBlock()
	var txResults []*abci.ExecTxResult

	for _, tx := range response.TxResults {
		// ignore checkpoint txs
		if tx.GasWanted == 0 {
			continue
		}

		require.NotEqual(d.t, tx.Code, uint32(0), tx.Log)
		txResults = append(txResults, tx)
	}

	return txResults
}

func (d *BabylonAppDriver) GetDriverAccountAddress() sdk.AccAddress {
	return sdk.AccAddress(d.SenderInfo.privKey.PubKey().Address())
}

func BlocksWithProofsToHeaderBytes(blocks []*datagen.BlockWithProofs) []bbn.BTCHeaderBytes {
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
	headers := BlocksWithProofsToHeaderBytes(blocks)

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
	headers := BlocksWithProofsToHeaderBytes([]*datagen.BlockWithProofs{block})

	d.SendTxWithMsgsFromDriverAccount(t, &btclighttypes.MsgInsertHeaders{
		Signer:  d.GetDriverAccountAddress().String(),
		Headers: headers,
	})

	return block
}

func blockWithProofsToActivationMessages(
	blockWithProofs *datagen.BlockWithProofs,
	senderAddr sdk.AccAddress,
) []sdk.Msg {
	msgs := []sdk.Msg{}

	for i, tx := range blockWithProofs.Transactions {
		// no coinbase tx
		if i == 0 {
			continue
		}

		msgs = append(msgs, &bstypes.MsgAddBTCDelegationInclusionProof{
			Signer:                  senderAddr.String(),
			StakingTxHash:           tx.TxHash().String(),
			StakingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(blockWithProofs.Proofs[i]),
		})
	}
	return msgs
}

// Activates all verified delegations in two blocks:
// 1. First block extends light client so that all stakers are confirmed
// 2. Second block activates all verified delegations
func (d *BabylonAppDriver) ActivateVerifiedDelegations(expectedVerifiedDelegations int) []*abci.ResponseFinalizeBlock {
	btcBlock, bbnBlock := d.IncludeVerifiedStakingTxInBTC(expectedVerifiedDelegations)
	acitvationMsgs := blockWithProofsToActivationMessages(btcBlock, d.GetDriverAccountAddress())
	return []*abci.ResponseFinalizeBlock{
		bbnBlock,
		d.SendTxWithMsgsFromDriverAccount(d.t, acitvationMsgs...),
	}
}

// IncludeVerifiedStakingTxInBTC extends light client so that all staking txs are confirmed (k deep).
// Returns the block with the transactions
func (d *BabylonAppDriver) IncludeVerifiedStakingTxInBTC(expectedVerifiedDelegations int) (btcBlock *datagen.BlockWithProofs, bbnBlock *abci.ResponseFinalizeBlock) {
	verifiedDelegations := d.GetVerifiedBTCDelegations(d.t)

	// Only verify number if requested
	if expectedVerifiedDelegations != 0 {
		require.Equal(d.t, len(verifiedDelegations), expectedVerifiedDelegations)
	}

	var transactions []*wire.MsgTx
	for _, del := range verifiedDelegations {
		stakingTx, _, err := bbn.NewBTCTxFromHex(del.StakingTxHex)
		require.NoError(d.t, err)
		transactions = append(transactions, stakingTx)
	}

	return d.IncludeTxsInBTCAndConfirm(transactions)
}

func (d *BabylonAppDriver) IncludeTxsInBTCAndConfirm(
	txs []*wire.MsgTx,
) (btcBlock *datagen.BlockWithProofs, bbnBlock *abci.ResponseFinalizeBlock) {
	btcCheckpointParams := d.GetBTCCkptParams(d.t)

	tip, _ := d.GetBTCLCTip()

	block := datagen.GenRandomBtcdBlockWithTransactions(d.r, txs, tip)
	headers := BlocksWithProofsToHeaderBytes([]*datagen.BlockWithProofs{block})

	confirmationBLocks := datagen.GenNEmptyBlocks(
		d.r,
		uint64(btcCheckpointParams.BtcConfirmationDepth),
		&block.Block.Header,
	)
	confirmationHeaders := BlocksWithProofsToHeaderBytes(confirmationBLocks)

	headers = append(headers, confirmationHeaders...)

	// extend our light client so that all stakers are confirmed
	bbnBlock = d.SendTxWithMsgsFromDriverAccount(d.t, &btclighttypes.MsgInsertHeaders{
		Signer:  d.GetDriverAccountAddress().String(),
		Headers: headers,
	})

	return block, bbnBlock
}

// BatchUnbondDelegations processes multiple unbondings in a single Babylon block.
// All unbonding BTC txs are included in the same BTC block, and all MsgBTCUndelegate
// messages are submitted before generating the Babylon block.
func (d *BabylonAppDriver) BatchUnbondDelegations(unbondings []*UnbondingInfo) {
	if len(unbondings) == 0 {
		return
	}

	// Collect all unbonding txs
	var unbondingTxs []*wire.MsgTx
	for _, ub := range unbondings {
		unbondingTxs = append(unbondingTxs, ub.UnbondingTx)
	}

	// Include all unbonding txs in a single BTC block and confirm
	btcCheckpointParams := d.GetBTCCkptParams(d.t)
	tip, _ := d.GetBTCLCTip()

	block := datagen.GenRandomBtcdBlockWithTransactions(d.r, unbondingTxs, tip)
	headers := BlocksWithProofsToHeaderBytes([]*datagen.BlockWithProofs{block})

	confirmationBlocks := datagen.GenNEmptyBlocks(
		d.r,
		uint64(btcCheckpointParams.BtcConfirmationDepth),
		&block.Block.Header,
	)
	confirmationHeaders := BlocksWithProofsToHeaderBytes(confirmationBlocks)
	headers = append(headers, confirmationHeaders...)

	// Insert BTC headers (this creates a Babylon block)
	d.SendTxWithMsgsFromDriverAccount(d.t, &btclighttypes.MsgInsertHeaders{
		Signer:  d.GetDriverAccountAddress().String(),
		Headers: headers,
	})

	// Now send all MsgBTCUndelegate messages (one per staker)
	// Each proof index is: 0 = coinbase, 1 = first unbonding tx, 2 = second, etc.
	for i, ub := range unbondings {
		stakingTxBz, err := bbn.SerializeBTCTx(ub.StakingTx)
		require.NoError(d.t, err)

		unbondingTxBytes, err := bbn.SerializeBTCTx(ub.UnbondingTx)
		require.NoError(d.t, err)

		// Proof index is i+1 because index 0 is the coinbase tx
		msg := &bstypes.MsgBTCUndelegate{
			Signer:                        ub.Staker.AddressString(),
			StakingTxHash:                 ub.StakingTxHash.String(),
			StakeSpendingTx:               unbondingTxBytes,
			StakeSpendingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(block.Proofs[i+1]),
			FundingTransactions:           [][]byte{stakingTxBz},
		}

		ub.Staker.SendMessage(msg)
	}

	// Generate the Babylon block that includes all unbonding messages
	d.GenerateNewBlockAssertExecutionSuccess()
}

func (d *BabylonAppDriver) IncludeTxsInBTC(txs []*wire.MsgTx) *datagen.BlockWithProofs {
	tip, _ := d.GetBTCLCTip()

	block := datagen.GenRandomBtcdBlockWithTransactions(d.r, txs, tip)
	headers := BlocksWithProofsToHeaderBytes([]*datagen.BlockWithProofs{block})

	// extend our light client so that all stakers are confirmed
	d.SendTxWithMsgsFromDriverAccount(d.t, &btclighttypes.MsgInsertHeaders{
		Signer:  d.GetDriverAccountAddress().String(),
		Headers: headers,
	})

	return block
}

func (d *BabylonAppDriver) GenerateNewBlockAssertExecutionSuccessWithResults() []*abci.ExecTxResult {
	response := d.GenerateNewBlock()

	for _, tx := range response.TxResults {
		// ignore checkpoint txs
		if tx.GasWanted == 0 {
			continue
		}

		require.Equal(d.t, tx.Code, uint32(0), tx.Log)
	}

	return response.TxResults
}

// ConfirmStakingTransactionOnBTC confirms staking transactions included in the
// provided messages on the simulated BTC chain. Afterwards, it fills inclusion
// proof in the provided messages. It is up to the caller to send the messages
// to the mempool.
func (d *BabylonAppDriver) ConfirmStakingTransactionOnBTC(
	msg []*bstypes.MsgCreateBTCDelegation,
) {
	require.NotEmpty(d.t, msg)

	btcCheckpointParams := d.GetBTCCkptParams(d.t)

	tip, _ := d.GetBTCLCTip()
	var transactions []*wire.MsgTx
	for _, m := range msg {
		stakingTx, err := bbn.NewBTCTxFromBytes(m.StakingTx)
		require.NoError(d.t, err)
		transactions = append(transactions, stakingTx)
	}

	block := datagen.GenRandomBtcdBlockWithTransactions(d.r, transactions, tip)

	headers := BlocksWithProofsToHeaderBytes([]*datagen.BlockWithProofs{block})

	confirmationBLocks := datagen.GenNEmptyBlocks(
		d.r,
		uint64(btcCheckpointParams.BtcConfirmationDepth),
		&block.Block.Header,
	)
	confirmationHeaders := BlocksWithProofsToHeaderBytes(confirmationBLocks)

	headers = append(headers, confirmationHeaders...)

	// extend our light client so that all stakers are confirmed
	d.SendTxWithMsgsFromDriverAccount(d.t, &btclighttypes.MsgInsertHeaders{
		Signer:  d.GetDriverAccountAddress().String(),
		Headers: headers,
	})

	// iterate over all transactions except coinbase, and set inclusion proof in message
	// to the proof from the block
	for i := 1; i < len(block.Transactions); i++ {
		msg[i-1].StakingTxInclusionProof = bstypes.NewInclusionProofFromSpvProof(block.Proofs[i])
	}
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

func (d *BabylonAppDriver) FinalizeCkptForEpoch(epochNumber uint64) {
	lastFinalizedEpoch := d.GetLastFinalizedEpoch()
	require.Equal(d.t, lastFinalizedEpoch+1, epochNumber)

	btckptParams := d.GetBTCCkptParams(d.t)
	d.GenCkptForEpoch(d.r, d.t, epochNumber)

	_, _ = d.ExtendBTCLcWithNEmptyBlocks(d.r, d.t, btckptParams.CheckpointFinalizationTimeout)

	lastFinalizedEpoch = d.GetLastFinalizedEpoch()
	require.Equal(d.t, lastFinalizedEpoch, epochNumber)
}

func (d *BabylonAppDriver) ProgressTillFirstBlockTheNextEpoch() {
	currentEpochNumber := d.GetEpoch().EpochNumber
	nextEpochNumber := currentEpochNumber + 1

	for currentEpochNumber < nextEpochNumber {
		d.GenerateNewBlock()
		currentEpochNumber = d.GetEpoch().EpochNumber
	}
}

func (d *BabylonAppDriver) WaitTillAllFpsJailed(t *testing.T) {
	for {
		activeFps := d.GetActiveFpsAtCurrentHeight(t)
		if len(activeFps) == 0 {
			break
		}
		d.GenerateNewBlock()
	}
}

// SendTxWithMsgsFromDriverAccount sends tx with msgs from driver account and asserts that
// execution was successful. It assumes that there will only be one tx in the block.
func (d *BabylonAppDriver) SendTxWithMsgsFromDriverAccount(
	t *testing.T,
	msgs ...sdk.Msg,
) *abci.ResponseFinalizeBlock {
	d.SendTxWithMessagesSuccess(
		t,
		d.SenderInfo,
		DefaultGasLimit,
		defaultFeeCoin,
		msgs...,
	)

	result := d.GenerateNewBlock()

	for _, rs := range result.TxResults {
		// our checkpoint transactions have 0 gas wanted, skip them to avoid confusing the
		// tests
		if rs.GasWanted == 0 {
			continue
		}

		// all executions should be successful
		require.Equal(t, rs.Code, uint32(0), rs.Log)
	}

	d.IncSeq()
	return result
}

// Function to initiate different type of senders

type NewAccountInfo struct {
	CreationMsg *banktypes.MsgSend
	PrivKey     *secp256k1.PrivKey
}

func accInfosToCreationMsgs(acInfos []*NewAccountInfo) []sdk.Msg {
	msgs := []sdk.Msg{}
	for _, acInfo := range acInfos {
		msgs = append(msgs, acInfo.CreationMsg)
	}
	return msgs
}

func (d *BabylonAppDriver) CreateSendingAccountMessage() *NewAccountInfo {
	accPrivKey := secp256k1.GenPrivKey()
	accPubKey := accPrivKey.PubKey()
	accAddress := sdk.AccAddress(accPubKey.Address())

	msgBankSend := banktypes.MsgSend{
		FromAddress: d.GetDriverAccountAddress().String(),
		ToAddress:   accAddress.String(),
		// 100 BBN, should be enough for most tests
		Amount: sdk.NewCoins(sdk.NewCoin("ubbn", sdkmath.NewInt(100000000))),
	}

	return &NewAccountInfo{
		CreationMsg: &msgBankSend,
		PrivKey:     accPrivKey,
	}
}

func (d *BabylonAppDriver) getAccountInfo(accAddress string) sdk.AccountI {
	add, err := sdk.AccAddressFromBech32(accAddress)
	require.NoError(d.t, err)
	return d.App.AccountKeeper.GetAccount(d.GetContextForLastFinalizedBlock(), add)
}

func (d *BabylonAppDriver) CreateNStakerAccounts(n int) []*Staker {
	// pre-condition
	require.True(d.t, n > 0)

	var bankMsgs []*NewAccountInfo
	for i := 0; i < n; i++ {
		bankMsgs = append(bankMsgs, d.CreateSendingAccountMessage())
	}

	d.SendTxWithMsgsFromDriverAccount(d.t, accInfosToCreationMsgs(bankMsgs)...)

	stakers := []*Staker{}
	for _, m := range bankMsgs {
		acc := d.getAccountInfo(m.CreationMsg.ToAddress)
		stakerPrivKey, err := btcec.NewPrivateKey()
		require.NoError(d.t, err)
		stakers = append(stakers, &Staker{
			SenderInfo: &SenderInfo{
				privKey:        m.PrivKey,
				sequenceNumber: acc.GetSequence(),
				accountNumber:  acc.GetAccountNumber(),
			},
			r:             d.r,
			t:             d.t,
			d:             d,
			app:           d.App,
			BTCPrivateKey: stakerPrivKey,
		})
	}

	return stakers
}

func (d *BabylonAppDriver) CreateNFinalityProviderAccounts(n int) []*FinalityProvider {
	var fpInfos []*NewAccountInfo
	for i := 0; i < n; i++ {
		fpInfos = append(fpInfos, d.CreateSendingAccountMessage())
	}

	d.SendTxWithMsgsFromDriverAccount(d.t, accInfosToCreationMsgs(fpInfos)...)

	fps := []*FinalityProvider{}
	for _, accInf := range fpInfos {
		acc := d.getAccountInfo(accInf.CreationMsg.ToAddress)

		btvPrivKey, err := btcec.NewPrivateKey()
		require.NoError(d.t, err)

		fps = append(fps, &FinalityProvider{
			SenderInfo: &SenderInfo{
				privKey:        accInf.PrivKey,
				sequenceNumber: acc.GetSequence(),
				accountNumber:  acc.GetAccountNumber(),
			},
			r:             d.r,
			t:             d.t,
			d:             d,
			app:           d.App,
			BTCPrivateKey: btvPrivKey,
			Description:   datagen.GenRandomDescription(d.r),
		})
	}

	return fps
}

// One sender for all covenants to simplify the tests
func (d *BabylonAppDriver) CreateCovenantSender() *CovenantSender {
	accInfo := d.CreateSendingAccountMessage()

	d.SendTxWithMsgsFromDriverAccount(d.t, accInfosToCreationMsgs([]*NewAccountInfo{accInfo})...)

	acc := d.getAccountInfo(accInfo.CreationMsg.ToAddress)

	return &CovenantSender{
		SenderInfo: &SenderInfo{
			privKey:        accInfo.PrivKey,
			sequenceNumber: acc.GetSequence(),
			accountNumber:  acc.GetAccountNumber(),
		},
		r:   d.r,
		t:   d.t,
		d:   d,
		app: d.App,
	}
}

func (d *BabylonAppDriver) GovPropAndVote(msgInGovProp sdk.Msg) (lastPropId uint64) {
	msgToSend := d.NewGovProp(msgInGovProp)
	d.SendTxWithMsgsFromDriverAccount(d.t, msgToSend)

	props := d.GovProposals()
	lastPropId = props[len(props)-1].Id

	d.GovVote(lastPropId)
	return lastPropId
}

func (d *BabylonAppDriver) GovPropWaitPass(msgInGovProp sdk.Msg) {
	propId := d.GovPropAndVote(msgInGovProp)

	for {
		prop := d.GovProposal(propId)

		if prop.Status == v1.ProposalStatus_PROPOSAL_STATUS_FAILED {
			d.t.Fatalf("prop %d failed due to: %s", propId, prop.FailedReason)
		}

		if prop.Status == v1.ProposalStatus_PROPOSAL_STATUS_PASSED {
			break
		}

		d.GenerateNewBlockAssertExecutionSuccess()
	}
}

func (d *BabylonAppDriver) BankSend(
	t *testing.T,
	from *SenderInfo,
	to sdk.AccAddress,
	coins sdk.Coins,
) {
	msgBankSend := banktypes.MsgSend{
		FromAddress: from.AddressString(),
		ToAddress:   to.String(),
		// 100 BBN, should be enough for most tests
		Amount: coins,
	}

	DefaultSendTxWithMessagesSuccess(t, d.App, from, &msgBankSend)
	from.IncSeq()
}

func (d *BabylonAppDriver) BankSendNative(
	t *testing.T,
	from *SenderInfo,
	to sdk.AccAddress,
	amt int64,
) {
	d.BankSend(t, d.SenderInfo, from.Address(), sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(amt))))
}

func (d *BabylonAppDriver) BankSendNativeFromSender(
	to sdk.AccAddress,
	amt int64,
) {
	d.BankSend(d.t, d.SenderInfo, to, sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(amt))))
}

func (d *BabylonAppDriver) MintNativeTo(
	recipient sdk.AccAddress,
	amt int64,
) {
	mintCoins := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(amt)))
	err := d.App.MintKeeper.MintCoins(d.Ctx(), mintCoins)
	require.NoError(d.t, err)

	err = d.App.BankKeeper.SendCoinsFromModuleToAccount(d.Ctx(), minttypes.ModuleName, recipient, mintCoins)
	require.NoError(d.t, err)
}
