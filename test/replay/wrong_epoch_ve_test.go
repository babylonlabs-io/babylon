package replay

import (
	"bytes"
	"context"
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/log"
	babylonApp "github.com/babylonlabs-io/babylon/v4/app"
	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/initialization"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btclighttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	ckpttypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	"github.com/boljen/go-bitmap"
	dbmc "github.com/cometbft/cometbft-db"
	cs "github.com/cometbft/cometbft/consensus"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cometlog "github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/mempool"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	sm "github.com/cometbft/cometbft/state"
	"github.com/cometbft/cometbft/store"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// validatorInfo holds all signing material for one validator
type validatorInfo struct {
	CometAddress []byte
	CometPrivKey ed25519.PrivKey
	BlsSigner    ckpttypes.BlsSigner
	ValAddress   sdk.ValAddress
}

// multiValDriver extends BabylonAppDriver for multi-validator scenarios.
// maliciousIdx is the index into validators of the validator that sorts
// first in CometBFT's canonical commit-info order (lowest CometBFT address
// among equal-power validators). Picking that validator as the attacker
// guarantees its wrong-epoch sig lands inside the seal loop's >2/3 prefix.
type multiValDriver struct {
	*BabylonAppDriver
	validators   []validatorInfo
	maliciousIdx int
}

func newMultiValDriver(r *rand.Rand, t *testing.T) *multiValDriver {
	dir := t.TempDir()
	expeditedVotingPeriod := blkTime + time.Second

	// Four validators with equal power. Seal threshold (PowerSum*3 > total*2)
	// requires 3 of 4 sigs, so 3 honest validators alone can clear quorum
	// without the malicious one. The malicious validator can therefore delay
	// its precommit and have its VE arrive post-quorum in production
	// which is the scenario this test models.
	valConfigs := []*initialization.NodeConfig{
		{Name: "val0", Pruning: "default", IsValidator: true},
		{Name: "val1", Pruning: "default", IsValidator: true},
		{Name: "val2", Pruning: "default", IsValidator: true},
		{Name: "val3", Pruning: "default", IsValidator: true},
	}

	chain, err := initialization.InitChain(
		chainID, dir, valConfigs,
		expeditedVotingPeriod*2, expeditedVotingPeriod, 1,
		[]*btclighttypes.BTCHeaderInfo{},
		300_000_000,
		&initialization.StartingBtcStakingParams{
			CovenantCommittee: bbn.NewBIP340PKsFromBTCPKs(pks),
			CovenantQuorum:    CovenantQuorum,
		},
	)
	require.NoError(t, err)

	// Load BLS signers and CometBFT keys for all validators
	var vals []validatorInfo
	for _, node := range chain.Nodes {
		if !node.IsValidator {
			continue
		}
		blsSigner, err := appsigner.InitBlsSigner(node.ConfigDir)
		require.NoError(t, err)

		filePV := privval.LoadFilePV(
			node.ConfigDir+"/config/priv_validator_key.json",
			node.ConfigDir+"/data/priv_validator_state.json",
		)

		accAddr := sdk.MustAccAddressFromBech32(node.PublicAddress)
		vals = append(vals, validatorInfo{
			CometAddress: filePV.Key.Address,
			CometPrivKey: ed25519.PrivKey(node.CometPrivKey),
			BlsSigner:    *blsSigner,
			ValAddress:   sdk.ValAddress(accAddr),
		})
	}

	// Boot the app using validator 0's config (same as normal driver)
	_, doc := getGenDoc(t, chain.Nodes[0].ConfigDir)
	genDoc, err := doc.ToGenesisDoc()
	require.NoError(t, err)

	state, err := sm.MakeGenesisState(genDoc)
	require.NoError(t, err)

	stateStore := sm.NewStore(dbmc.NewMemDB(), sm.StoreOptions{DiscardABCIResponses: false})
	require.NoError(t, stateStore.Save(state))

	blsSigner0, _ := appsigner.InitBlsSigner(chain.Nodes[0].ConfigDir)
	appOptions := NewAppOptionsWithFlagHome(chain.Nodes[0].ConfigDir)
	baseAppOptions := server.DefaultBaseappOptions(appOptions)

	tmpApp := babylonApp.NewBabylonApp(
		log.NewNopLogger(), dbm.NewMemDB(), nil, true,
		map[int64]bool{}, 0, blsSigner0, appOptions,
		babylonApp.EmptyWasmOpts, baseAppOptions...,
	)

	cmtApp := server.NewCometABCIWrapper(tmpApp)
	proxyConn := proxy.NewMultiAppConn(proxy.NewLocalClientCreator(cmtApp), proxy.NopMetrics())
	require.NoError(t, proxyConn.Start())

	blockStore := store.NewBlockStore(dbmc.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore, cometlog.TestingLogger(), proxyConn.Consensus(),
		&mempool.NopMempool{}, sm.EmptyEvidencePool{}, blockStore,
	)

	hs := cs.NewHandshaker(stateStore, state, blockStore, genDoc)
	hs.SetLogger(cometlog.TestingLogger())
	require.NoError(t, hs.Handshake(proxyConn))

	state, err = stateStore.Load()
	require.NoError(t, err)

	valPrivKey := secp256k1.PrivKey{Key: chain.Nodes[0].PrivateKey}

	driver := &BabylonAppDriver{
		r: r, t: t, App: tmpApp, BlsSigner: *blsSigner0,
		SenderInfo: &SenderInfo{privKey: &valPrivKey, sequenceNumber: 1, accountNumber: 0},
		BlockExec: blockExec, BlockStore: blockStore, StateStore: stateStore,
		NodeDir: chain.Nodes[0].ConfigDir, CometAddress: vals[0].CometAddress,
		ValAddress: vals[0].ValAddress, CometPrivKey: vals[0].CometPrivKey,
		CurrentTime: time.Now(),
	}

	// Pick the validator with the lowest CometBFT address. With uniform
	// voting power, CometBFT's canonical commit-info order reduces to
	// address ascending, so this validator is processed first by the seal
	// loop and its sig is folded into the aggregate before the loop breaks.
	maliciousIdx := 0
	for i := 1; i < len(vals); i++ {
		if bytes.Compare(vals[i].CometAddress, vals[maliciousIdx].CometAddress) < 0 {
			maliciousIdx = i
		}
	}

	return &multiValDriver{BabylonAppDriver: driver, validators: vals, maliciousIdx: maliciousIdx}
}

// generateBlockWithAllValidators builds a block where every validator submits
// a vote extension. If maliciousIdx >= 0, that validator signs a VE for
// wrongEpoch instead of the real one. epochBoundaryHeight is the last block of
// the current epoch (0 means no BLS VEs are needed at this height).
func (md *multiValDriver) generateBlockWithAllValidators(t *testing.T, maliciousIdx int, wrongEpoch uint64, epochBoundaryHeight int64) {
	correctEpochNum := uint64(1) // epoch 1 for this test
	d := md.BabylonAppDriver
	lastState, err := d.StateStore.Load()
	require.NoError(t, err)

	var lastCommit *cmttypes.ExtendedCommit
	if lastState.LastBlockHeight == 0 {
		lastCommit = &cmttypes.ExtendedCommit{}
	} else {
		lastCommit = d.BlockStore.LoadBlockExtendedCommit(lastState.LastBlockHeight)
		require.NotNil(t, lastCommit)
	}

	block, err := d.BlockExec.CreateProposalBlock(
		context.Background(), lastState.LastBlockHeight+1,
		lastState, lastCommit, md.validators[0].CometAddress,
	)
	require.NoError(t, err)

	blockID, partSet := getBlockId(t, block)

	newTime := d.CurrentTime.Add(blkTime)
	validators := lastState.Validators
	extendedSignatures := make([]cmttypes.ExtendedCommitSig, len(validators.Validators))

	isEpochBoundary := epochBoundaryHeight > 0 && block.Height == epochBoundaryHeight

	for i, val := range validators.Validators {
		// Match the CometBFT address back to our validatorInfo entry.
		var vi *validatorInfo
		var viIdx int
		for j := range md.validators {
			if bytes.Equal(md.validators[j].CometAddress, val.Address.Bytes()) {
				vi = &md.validators[j]
				viIdx = j
				break
			}
		}

		if vi == nil {
			extendedSignatures[i] = cmttypes.ExtendedCommitSig{
				CommitSig: cmttypes.CommitSig{BlockIDFlag: cmttypes.BlockIDFlagAbsent},
			}
			continue
		}

		var extension []byte

		if isEpochBoundary {
			// At an epoch boundary every validator needs a VE signed with its
			// own BLS key. The app's ExtendVote only signs with validator 0's
			// key, so the test does this manually for the others.
			var bhash ckpttypes.BlockHash
			err := bhash.Unmarshal(block.Hash())
			require.NoError(t, err)

			epochNum := correctEpochNum
			isMalicious := viIdx == maliciousIdx

			if isMalicious {
				epochNum = wrongEpoch
				t.Logf("validator %d signs VE for wrong epoch %d at height %d (real epoch is %d)",
					viIdx, wrongEpoch, block.Height, correctEpochNum)
			}

			// Sign with this validator's real BLS key.
			signBytes := ckpttypes.GetSignBytes(epochNum, bhash)
			blsSig, err := vi.BlsSigner.SignMsgWithBls(signBytes)
			require.NoError(t, err)

			ve := &ckpttypes.VoteExtension{
				Signer:           vi.ValAddress.String(),
				ValidatorAddress: sdk.ValAddress(val.Address).String(),
				BlockHash:        &bhash,
				EpochNum:         epochNum,
				Height:           uint64(block.Height),
				BlsSig:           &blsSig,
			}

			extension, err = ve.Marshal()
			require.NoError(t, err)
		} else {
			// Off the epoch boundary, the real ExtendVote returns an empty VE.
			extension = []byte{}
		}

		// Sign the CometBFT extension envelope with this validator's key.
		extensionSig := signVoteExtension(t, extension, uint64(block.Height), vi.CometPrivKey)

		extendedSignatures[i] = cmttypes.ExtendedCommitSig{
			CommitSig: cmttypes.CommitSig{
				BlockIDFlag:      cmttypes.BlockIDFlagCommit,
				ValidatorAddress: val.Address,
				Timestamp:        newTime,
				Signature:        []byte("test"),
			},
			Extension:          extension,
			ExtensionSignature: extensionSig,
		}
	}
	d.CurrentTime = newTime

	commit := &cmttypes.ExtendedCommit{
		Height: block.Height, Round: 0, BlockID: blockID,
		ExtendedSignatures: extendedSignatures,
	}

	accepted, err := d.BlockExec.ProcessProposal(block, lastState)
	require.NoError(t, err)
	require.True(t, accepted, "ProcessProposal should accept")

	state, err := d.BlockExec.ApplyVerifiedBlock(lastState, blockID, block)
	require.NoError(t, err)
	require.NotNil(t, state)

	d.BlockStore.SaveBlockWithExtendedCommit(block, partSet, commit)
}

// TestWrongEpochVoteExtensionRejectedAfterFix is the regression test for the
// wrong-epoch vote-extension attack. Four equal-power validators run; three
// honest validators alone clear the >2/3 quorum, so the fourth (malicious)
// validator's vote extension can arrive post-quorum and bypass the
// CometBFT-side VerifyVoteExtension via the late-precommit path
// (cometbft#2361). The malicious validator's address is chosen as the
// lowest of the four so it sorts first in canonical commit-info order and
// would be processed before the seal loop's >2/3 break — i.e. it gets the
// best chance of being folded into the BLS aggregate.
//
// Pre-fix, the proposer-side VerifyVoteExtension in
// x/checkpointing/prepare/proposal.go did not re-check ve.EpochNum against the
// epoch the proposer was building a checkpoint for, so the malicious VE was
// accepted, its sig was aggregated, and VerifyRawCheckpoint over the sealed
// aggregate failed against canonical sign-bytes.
//
// Post-fix, the proposer-side check rejects the wrong-epoch VE before
// aggregation, so:
//   - the checkpoint still seals (the three honest validators clear quorum),
//   - the sealed aggregate verifies against canonical sign-bytes,
//   - the malicious validator's bit in the bitmap stays unset.
//
// Nothing in the checkpointing or BLS code paths is mocked. The Babylon app,
// CheckpointingKeeper, BLS signers, and CometBFT BlockExecutor are all real.
// The harness uses NopMempool and EmptyEvidencePool, which are unrelated to
// the code under test.
func TestWrongEpochVoteExtensionRejectedAfterFix(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	md := newMultiValDriver(r, t)

	require.Equal(t, 4, len(md.validators))
	t.Logf("initialized chain with %d validators; malicious index = %d", len(md.validators), md.maliciousIdx)

	md.generateBlockWithAllValidators(t, -1, 0, 0)

	epoch := md.GetEpoch()
	require.Equal(t, uint64(1), epoch.EpochNumber)
	lastBlockOfEpoch1 := epoch.FirstBlockHeight + epoch.CurrentEpochInterval - 1
	t.Logf("epoch 1: firstBlock=%d, interval=%d, lastBlock=%d",
		epoch.FirstBlockHeight, epoch.CurrentEpochInterval, lastBlockOfEpoch1)

	for md.GetLastFinalizedBlock().Height < lastBlockOfEpoch1-1 {
		md.generateBlockWithAllValidators(t, -1, 0, 0)
	}
	t.Log("step 1: advanced to one block before the epoch boundary")

	// At the last block of epoch 1, the malicious validator submits a
	// wrong-epoch VE. In production it would arrive post-quorum (the three
	// honest validators have already crossed >2/3), so CometBFT includes it
	// in the commit info without calling VerifyVoteExtension.
	wrongEpoch := uint64(999999)
	md.generateBlockWithAllValidators(t, md.maliciousIdx, wrongEpoch, int64(lastBlockOfEpoch1))
	t.Log("step 2: built last block of epoch 1; malicious validator contributed a wrong-epoch VE")

	// First block of epoch 2: PrepareProposal reads the commit info, builds
	// the checkpoint, and injects it. ProcessProposal accepts, PreBlocker
	// seals.
	md.generateBlockWithAllValidators(t, -1, 0, 0)

	epoch = md.GetEpoch()
	require.Equal(t, uint64(2), epoch.EpochNumber)
	t.Log("step 3: first block of epoch 2 produced; checkpoint for epoch 1 is sealed")

	ckpt := md.GetCheckpoint(t, 1)
	require.Equal(t, ckpttypes.Sealed, ckpt.Status)
	require.Equal(t, uint64(1), ckpt.Ckpt.EpochNum)
	t.Log("step 4: checkpoint for epoch 1 has status Sealed")

	// Post-fix: VerifyRawCheckpoint passes because the malicious VE was
	// rejected by the proposer-side epoch check before aggregation.
	require.NoError(t,
		md.App.CheckpointingKeeper.VerifyRawCheckpoint(md.Ctx(), ckpt.Ckpt),
		"VerifyRawCheckpoint should pass: the malicious VE must have been excluded")
	t.Log("step 5: VerifyRawCheckpoint passes against canonical sign-bytes")

	// The malicious validator's bit in the bitmap must be unset. The bitmap
	// is indexed by the validator's position in the epoch's BLS validator
	// set (epoching.ValidatorSet, sorted by ValAddress ascending), which is
	// not necessarily the same as md.maliciousIdx (which indexed into
	// md.validators).
	valSet := md.App.EpochingKeeper.GetValidatorSet(md.Ctx(), 1)
	maliciousValAddr := md.validators[md.maliciousIdx].ValAddress
	_, bitmapIdx, err := valSet.FindValidatorWithIndex(maliciousValAddr)
	require.NoError(t, err)
	require.False(t, bitmap.Get(ckpt.Ckpt.Bitmap, bitmapIdx),
		"malicious validator's bitmap bit must be 0: its wrong-epoch VE should not have been aggregated")
	t.Logf("step 6: malicious validator (bitmap index %d) excluded from the aggregate", bitmapIdx)
}
