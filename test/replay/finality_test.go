package replay

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/CosmWasm/wasmd/x/wasm/ioutils"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/types"
	ftypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func FuzzJailing(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 5)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		numFinalityProviders := datagen.RandomInRange(r, 2, 3)
		numDelPerFp := 2
		driverTempDir := t.TempDir()
		replayerTempDir := t.TempDir()
		driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
		driver.GenerateNewBlock()

		scenario := NewStandardScenario(driver)
		scenario.InitScenario(numFinalityProviders, numDelPerFp)

		// we do not have voting in this test, so wait until all fps are jailed
		driver.WaitTillAllFpsJailed(t)
		driver.GenerateNewBlock()
		activeFps := driver.GetActiveFpsAtCurrentHeight(t)
		require.Equal(t, 0, len(activeFps))

		// Replay all the blocks from driver and check appHash
		replayer := NewBlockReplayer(t, replayerTempDir)
		replayer.ReplayBlocks(t, driver.GetFinalizedBlocks())
		// after replay we should have the same apphash
		require.Equal(t, driver.GetLastState().LastBlockHeight, replayer.LastState.LastBlockHeight)
		require.Equal(t, driver.GetLastState().AppHash, replayer.LastState.AppHash)
	})
}

func TestResumeFinalityOfSlashedFp(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlock()

	scn := NewStandardScenario(d)
	scn.InitScenario(2, 1) // 2 fps, 1 del each

	numBlocksFinalized := uint64(2)
	lastVotedBlkHeight := scn.FinalityFinalizeBlocksAllVotes(scn.activationHeight, numBlocksFinalized)

	// one fp continues to vote, but the one to be jailed one does not vote
	indexSlashFp := 1
	lastFinalizedBlkHeight := lastVotedBlkHeight

	// vote only with one fp, to halt finality
	for i := uint64(0); i < numBlocksFinalized; i++ {
		lastVotedBlkHeight++
		for _, fp := range scn.finalityProviders[:indexSlashFp] {
			fp.CastVote(lastVotedBlkHeight)
		}

		d.GenerateNewBlock()

		bl := d.GetIndexedBlock(lastVotedBlkHeight)
		require.Equal(t, bl.Finalized, false)
	}

	// verifes that the fp is still not jailed or slashed
	slashFp := scn.finalityProviders[indexSlashFp]
	fp := d.GetFp(*slashFp.BTCPublicKey())
	require.False(t, fp.Jailed)
	require.False(t, fp.IsSlashed())

	vp := d.App.FinalityKeeper.GetVotingPower(d.Ctx(), *slashFp.BTCPublicKey(), lastFinalizedBlkHeight)
	require.NotZero(t, vp)

	badBlock := d.GetIndexedBlock(lastVotedBlkHeight - 1)
	goodBlock := d.GetIndexedBlock(lastVotedBlkHeight)
	// fp slashed with bogus vote
	slashFp.CastVoteForHash(lastVotedBlkHeight, badBlock.AppHash)
	slashFp.CastVoteForHash(lastVotedBlkHeight, goodBlock.AppHash)

	d.GenerateNewBlockAssertExecutionSuccess()

	slashedFp := d.GetFp(*slashFp.BTCPublicKey())
	require.False(t, slashedFp.Jailed)
	require.True(t, slashedFp.IsSlashed())

	// send gov proposal to resume finality
	prop := ftypes.MsgResumeFinalityProposal{
		Authority:     appparams.AccGov.String(),
		FpPksHex:      []string{slashedFp.BtcPk.MarshalHex()},
		HaltingHeight: uint32(lastFinalizedBlkHeight + 1), // fp voted in the last finalized
	}
	d.GovPropWaitPass(&prop)

	d.GenerateNewBlock()
	// check that the blocks got finalized
	for blkHeight := lastFinalizedBlkHeight + 1; blkHeight <= lastVotedBlkHeight; blkHeight++ {
		bl := d.GetIndexedBlock(blkHeight)
		require.Equal(t, bl.Finalized, true)

		vp := d.App.FinalityKeeper.GetVotingPower(d.Ctx(), *slashFp.BTCPublicKey(), blkHeight)
		require.Zero(t, vp)
	}

	// the fp in the voting power distribution cache should still be marked as slashed
	vpDstCache := d.GetVotingPowerDistCache(d.GetLastFinalizedBlock().Height)
	for _, vpFp := range vpDstCache.FinalityProviders {
		if vpFp.BtcPk.Equals(slashFp.BTCPublicKey()) {
			require.True(d.t, vpFp.IsSlashed)
			require.Zero(d.t, vpFp.TotalBondedSat)
			continue
		}

		require.False(d.t, vpFp.IsJailed)
		require.False(d.t, vpFp.IsSlashed)
		require.NotZero(d.t, vpFp.TotalBondedSat)
	}

	// continue to be slashed status in btcstaking
	slashedFp = d.GetFp(*slashFp.BTCPublicKey())
	require.True(t, slashedFp.IsSlashed())
}

func TestResumeFinalityOfJailedSlashedFp(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlock()

	scn := NewStandardScenario(d)
	scn.InitScenario(2, 1) // 2 fps, 1 del each

	// finalize first 2 blocks, where both vote
	numBlocksFinalized := uint64(2)
	lastVotedBlkHeight := scn.FinalityFinalizeBlocksAllVotes(scn.activationHeight, numBlocksFinalized)

	// one fp continues to vote, but the one to be jailed one does not vote
	indexSlashFp := 1
	jailedFp := scn.finalityProviders[indexSlashFp]
	lastFinalizedBlkHeight := lastVotedBlkHeight

	for {
		lastVotedBlkHeight++
		for _, fp := range scn.finalityProviders[:indexSlashFp] {
			fp.CastVote(lastVotedBlkHeight)
		}

		d.GenerateNewBlock()

		bl := d.GetIndexedBlock(lastVotedBlkHeight)
		require.Equal(t, bl.Finalized, false)

		fp := d.GetFp(*jailedFp.BTCPublicKey())
		if fp.Jailed {
			break
		}
	}

	// fp is slashed, sending bogus vote is not enough since the fp
	// is jailed, new votes are no accepted. It is needed to send a
	// selective slash with one of the BTC delegations stk txs
	jailedFp.SendSelectiveSlashingEvidence()
	d.GenerateNewBlock()

	slashedFp := d.GetFp(*jailedFp.BTCPublicKey())
	require.True(t, slashedFp.IsSlashed())

	// send gov proposal to resume finality
	prop := ftypes.MsgResumeFinalityProposal{
		Authority:     appparams.AccGov.String(),
		FpPksHex:      []string{slashedFp.BtcPk.MarshalHex()},
		HaltingHeight: uint32(lastFinalizedBlkHeight + 1), // fp voted in the last finalized
	}
	d.GovPropWaitPass(&prop)

	d.GenerateNewBlock()
	// check that the blocks got finalized
	for blkHeight := lastFinalizedBlkHeight + 1; blkHeight <= lastVotedBlkHeight; blkHeight++ {
		bl := d.GetIndexedBlock(blkHeight)
		require.Equal(t, bl.Finalized, true)

		vp := d.App.FinalityKeeper.GetVotingPower(d.Ctx(), *jailedFp.BTCPublicKey(), blkHeight)
		require.Zero(t, vp)
	}

	// the fp in the voting power distribution cache should still be marked as slashed
	vpDstCache := d.GetVotingPowerDistCache(d.GetLastFinalizedBlock().Height)
	for _, vpFp := range vpDstCache.FinalityProviders {
		if vpFp.BtcPk.Equals(jailedFp.BTCPublicKey()) {
			require.True(d.t, vpFp.IsSlashed)
			require.Zero(d.t, vpFp.TotalBondedSat)
			continue
		}

		require.False(d.t, vpFp.IsJailed)
		require.False(d.t, vpFp.IsSlashed)
		require.NotZero(d.t, vpFp.TotalBondedSat)
	}

	// continue to be slashed status in btcstaking
	slashedFp = d.GetFp(*jailedFp.BTCPublicKey())
	require.True(t, slashedFp.IsSlashed())
}

func TestMissingSignInfoNewlyActiveFpSet(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	_, finalityK := d.App.BTCStakingKeeper, d.App.FinalityKeeper

	totalNumFps := 6

	// sets the max active fps to total - 1
	fParams := finalityK.GetParams(d.Ctx())
	fParams.MaxActiveFinalityProviders = uint32(totalNumFps - 1)
	err := finalityK.SetParams(d.Ctx(), fParams)
	require.NoError(t, err)

	scn := NewStandardScenario(d)
	scn.InitScenario(totalNumFps, 1) // each fp has only one del

	d.GenerateNewBlockAssertExecutionSuccess()

	// finalize blocks with one less FP vote
	fpsToVote := scn.FpMapBtcPkHex()

	dc := d.GetVotingPowerDistCache(scn.activationHeight)
	dc.ApplyActiveFinalityProviders(fParams.MaxActiveFinalityProviders)

	fpActiveNotVoting := dc.FinalityProviders[dc.NumActiveFps-1]
	delete(fpsToVote, fpActiveNotVoting.BtcPk.MarshalHex())

	lastFinalizedHeight := scn.FinalityFinalizeBlocks(scn.activationHeight, 4, fpsToVote)

	d.GenerateNewBlockAssertExecutionSuccess()

	// check the voting power distribution cache has the inactive FP
	dc = d.GetVotingPowerDistCache(lastFinalizedHeight)
	require.Equal(t, dc.NumActiveFps, fParams.MaxActiveFinalityProviders)
	require.Len(t, dc.FinalityProviders, totalNumFps)

	dc.ApplyActiveFinalityProviders(fParams.MaxActiveFinalityProviders)

	dcActives := dc.GetActiveFinalityProviderSet()
	require.Len(t, dcActives, int(fParams.MaxActiveFinalityProviders))
	dcInactives := dc.GetInactiveFinalityProviderSet()
	require.Len(t, dcInactives, 1)

	// inactive FP should not have signing info
	inactiveFp := dc.FinalityProviders[dc.NumActiveFps]
	inactivePkHex := inactiveFp.BtcPk.MarshalHex()
	inactiveSigInfo, err := finalityK.SigningInfo(d.Ctx(), &ftypes.QuerySigningInfoRequest{
		FpBtcPkHex: inactivePkHex,
	})
	require.EqualError(t, err, status.Errorf(codes.NotFound, "SigningInfo not found for the finality provider %s", inactivePkHex).Error())
	require.Nil(t, inactiveSigInfo)

	// vote for a few more blocks deleting one more fp to halt finality
	fpToNotVote := dc.FinalityProviders[0]
	delete(fpsToVote, fpToNotVote.BtcPk.MarshalHex())

	blkHeightStartVote := lastFinalizedHeight + 1
	blkHeightLastVoted := blkHeightStartVote + 2
	for blkHeight := blkHeightStartVote; blkHeight <= blkHeightLastVoted; blkHeight++ {
		scn.FinalityCastVotes(blkHeight, fpsToVote)
		d.GenerateNewBlockAssertExecutionSuccess()

		bl := d.GetIndexedBlock(blkHeight)
		require.Equal(t, false, bl.Finalized)
	}

	// send gov proposal to resume finality and bring the inactive without signing info to active
	prop := ftypes.MsgResumeFinalityProposal{
		Authority:     appparams.AccGov.String(),
		FpPksHex:      []string{fpToNotVote.BtcPk.MarshalHex()},
		HaltingHeight: uint32(blkHeightStartVote),
	}
	d.GovPropWaitPass(&prop)

	// check that the signing info of the inactive fp got on
	inactiveSigInfo, err = finalityK.SigningInfo(d.Ctx(), &ftypes.QuerySigningInfoRequest{
		FpBtcPkHex: inactivePkHex,
	})
	require.NoError(t, err)
	require.NotNil(t, inactiveSigInfo)

	// check the heights got finalized
	for blkHeight := blkHeightStartVote; blkHeight <= blkHeightLastVoted; blkHeight++ {
		bl := d.GetIndexedBlock(blkHeight)
		require.Equal(t, true, bl.Finalized)
	}

	// verify the vp dst cache has the inactive fp as active
	dc = d.GetVotingPowerDistCache(uint64(d.Ctx().BlockHeader().Height))
	require.Equal(t, dc.NumActiveFps, fParams.MaxActiveFinalityProviders)
	require.Len(t, dc.FinalityProviders, totalNumFps)
	dc.ApplyActiveFinalityProviders(dc.NumActiveFps)

	// inactive -> active
	activeFps := dc.GetActiveFinalityProviderSet()
	_, isActive := activeFps[inactivePkHex]
	require.True(t, isActive)

	// vote and finalize a few more blocks
	scn.FinalityFinalizeBlocksAllVotes(blkHeightLastVoted+1, 2)
}

func TestOnlyBabylonFpCanCommitRandomness(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)

	const consumerID1 = "consumer1"

	// 1. Set up mock IBC clients for each consumer before registering consumers
	ctx := driver.App.BaseApp.NewContext(false)
	driver.App.IBCKeeper.ClientKeeper.SetClientState(ctx, consumerID1, &ibctmtypes.ClientState{})
	driver.GenerateNewBlock()

	// 2. Register consumers
	consumer1 := driver.RegisterConsumer(r, consumerID1)
	require.NotNil(t, consumer1)

	// Create a Babylon FP (registered without consumer ID)
	consumerFp := driver.CreateNFinalityProviderAccounts(1)[0]
	consumerFp.RegisterFinalityProvider(consumerID1)
	driver.GenerateNewBlock()

	consumerFp.CommitRandomness()

	msg := fmt.Sprintf("failed to execute message; message index: 0: the finality provider with BTC PK %s is not a Babylon Genesis finality provider: the public randomness list is invalid", consumerFp.BTCPublicKey().MarshalHex())

	txResults := driver.GenerateNewBlockAssertExecutionFailure()
	require.Len(t, txResults, 1)
	require.Equal(t, txResults[0].Code, uint32(1106))
	require.Contains(t, txResults[0].Log, msg)
}

func TestFinalityVotOnConsumerOnContract(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)
	driver.GenerateNewBlock()

	const bsnId = "bsn1"
	const wampath = "finality.wasm"

	babylonFpSender := driver.CreateNFinalityProviderAccounts(1)[0]
	babylonFpSender.RegisterFinalityProvider("")

	consumerFpSender := driver.CreateNFinalityProviderAccounts(1)[0]
	require.NotNil(t, consumerFpSender)

	consumerFPPK := consumerFpSender.BTCPublicKey()

	contractAddress := driver.StoreAndDeployContract(bsnId, wampath, consumerFPPK)
	driver.RegisterConsumer(r, bsnId, contractAddress)

	// register consumer fp as finality provider
	consumerFpSender.RegisterFinalityProvider(bsnId)
	driver.GenerateNewBlockAssertExecutionSuccess()

	rinfo := driver.CommitRandomnessForConsumerFP(consumerFpSender, contractAddress)

	// need to finalize 2 epochs
	driver.FinializeCkptForEpoch(1)
	driver.ProgressTillFirstBlockTheNextEpoch()
	driver.FinializeCkptForEpoch(2)

	driver.CastConsumerVoteForHeight(consumerFpSender, contractAddress, 1, rinfo)
	driver.CastConsumerVoteForHeight(consumerFpSender, contractAddress, 2, rinfo)
}

func (d *BabylonAppDriver) ConsumerVoteForHeight(
	consumerFpSender *FinalityProvider,
	contractAddress string,
	height uint64,
	rinfo *datagen.RandListInfo,
) []byte {
	hash := [32]byte{}

	castVoteMsg, err := datagen.NewMsgAddFinalitySig(
		consumerFpSender.AddressString(),
		consumerFpSender.BTCPrivateKey,
		signingcontext.FpFinVoteContextV0(d.App.ChainID(), contractAddress),
		1,
		height,
		rinfo,
		hash[:],
	)
	require.NoError(d.t, err)

	proof := Proof{
		Total:    uint64(castVoteMsg.Proof.Total),
		Index:    uint64(castVoteMsg.Proof.Index),
		LeafHash: castVoteMsg.Proof.LeafHash,
		Aunts:    castVoteMsg.Proof.Aunts,
	}

	sm := SubmitFinalitySignatureMsg{
		SubmitFinalitySignature: SubmitFinalitySignatureMsgParams{
			FpPubkeyHex: consumerFpSender.BTCPublicKey().MarshalHex(),
			Height:      height,
			PubRand:     castVoteMsg.PubRand.MustMarshal(),
			Proof:       proof,
			BlockHash:   hash[:],
			Signature:   castVoteMsg.FinalitySig.MustMarshal(),
		},
	}

	smBytes, err := json.Marshal(sm)
	require.NoError(d.t, err)

	return smBytes
}

func (d *BabylonAppDriver) RandomnessCommitment(
	consumerFpSender *FinalityProvider,
	contractAddress string,
) ([]byte, *datagen.RandListInfo) {
	rinfo, m, err := datagen.GenRandomMsgCommitPubRandList(
		d.r,
		consumerFpSender.BTCPrivateKey,
		signingcontext.FpRandCommitContextV0(d.App.ChainID(), contractAddress),
		1,
		10000,
	)
	require.NoError(d.t, err)

	rndCommit := `{
		"commit_public_randomness": {
			"fp_pubkey_hex": "` + consumerFpSender.BTCPublicKey().MarshalHex() + `",
			"start_height": 1,
			"num_pub_rand": 10000,
			"commitment": "` + base64.StdEncoding.EncodeToString(rinfo.Commitment) + `",
			"signature": "` + base64.StdEncoding.EncodeToString(m.Sig.MustMarshal()) + `"
		}
	}`

	return []byte(rndCommit), rinfo
}

func (driver *BabylonAppDriver) StoreAndDeployContract(
	bsnId string,
	wampath string,
	allowedFpPubKey *types.BIP340PubKey,
) string {

	wasmCode, err := os.ReadFile(wampath)
	require.NoError(driver.t, err)

	gzippedCode, err := ioutils.GzipIt(wasmCode)
	require.NoError(driver.t, err)

	wasmParams := driver.App.WasmKeeper.GetParams(driver.Ctx())

	wasmParams.CodeUploadAccess = wasmtypes.AccessConfig{
		Permission: wasmtypes.AccessTypeEverybody,
		Addresses:  []string{appparams.AccGov.String()},
	}

	driver.GovPropWaitPass(&wasmtypes.MsgUpdateParams{
		Authority: appparams.AccGov.String(),
		Params:    wasmParams,
	})

	driver.GenerateNewBlock()

	driver.SendTxWithMsgsFromDriverAccount(driver.t, &wasmtypes.MsgStoreCode{
		Sender:       driver.AddressString(),
		WASMByteCode: gzippedCode,
	})

	msg := `{
		"admin": "` + driver.AddressString() + `",
		"bsn_id": "` + bsnId + `",
		"min_pub_rand": 150,
		"rate_limiting_interval": 100,
		"max_msgs_per_interval": 100,
		"bsn_activation_height": 0,
		"finality_signature_interval": 1,
		"allowed_finality_providers": [
			"` + allowedFpPubKey.MarshalHex() + `"
		]
	}`

	txResults := driver.SendTxWithMsgsFromDriverAccounGetResults(driver.t, &wasmtypes.MsgInstantiateContract{
		Sender: driver.AddressString(),
		CodeID: 1,
		Msg:    []byte(msg),
		Label:  "finality",
	})

	require.Len(driver.t, txResults, 1)

	// find event with "_contract_address" key and return the value
	contractAddress := ""
	for _, event := range txResults[0].Events {
		if event.Type == "instantiate" {
			for _, attr := range event.Attributes {
				if attr.Key == "_contract_address" {
					contractAddress = string(attr.Value)
					break
				}
			}
		}
	}

	require.NotEmpty(driver.t, contractAddress)

	return contractAddress
}

func (driver *BabylonAppDriver) CommitRandomnessForConsumerFP(
	consumerFpSender *FinalityProvider,
	contractAddress string,
) *datagen.RandListInfo {
	rndCommit, rinfo := driver.RandomnessCommitment(consumerFpSender, contractAddress)

	driver.SendTxWithMsgsFromDriverAccount(driver.t, &wasmtypes.MsgExecuteContract{
		Sender:   driver.AddressString(),
		Contract: contractAddress,
		Msg:      rndCommit,
	})

	return rinfo
}

func (driver *BabylonAppDriver) CastConsumerVoteForHeight(
	consumerFpSender *FinalityProvider,
	contractAddress string,
	height uint64,
	rinfo *datagen.RandListInfo,
) {
	smBytes := driver.ConsumerVoteForHeight(consumerFpSender, contractAddress, height, rinfo)

	driver.SendTxWithMsgsFromDriverAccount(driver.t, &wasmtypes.MsgExecuteContract{
		Sender:   driver.AddressString(),
		Contract: contractAddress,
		Msg:      smBytes,
	})
}

type Proof struct {
	Total    uint64   `json:"total"`
	Index    uint64   `json:"index"`
	LeafHash []byte   `json:"leaf_hash"`
	Aunts    [][]byte `json:"aunts"`
}

type SubmitFinalitySignatureMsgParams struct {
	FpPubkeyHex string `json:"fp_pubkey_hex"`
	Height      uint64 `json:"height"`
	PubRand     []byte `json:"pub_rand"`
	Proof       Proof  `json:"proof"`
	BlockHash   []byte `json:"block_hash"`
	Signature   []byte `json:"signature"`
}

type SubmitFinalitySignatureMsg struct {
	SubmitFinalitySignature SubmitFinalitySignatureMsgParams `json:"submit_finality_signature"`
}
