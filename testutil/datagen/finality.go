package datagen

import (
	"errors"
	"math/rand"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/cometbft/cometbft/crypto/merkle"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/crypto/eots"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

type RandListInfo struct {
	SRList     []*eots.PrivateRand
	PRList     []bbn.SchnorrPubRand
	Commitment []byte
	ProofList  []*merkle.Proof
}

func GenRandomPubRandList(r *rand.Rand, numPubRand uint64) (*RandListInfo, error) {
	// generate a list of secret/public randomness
	srList := []*eots.PrivateRand{}
	prList := []bbn.SchnorrPubRand{}
	for i := uint64(0); i < numPubRand; i++ {
		eotsSR, eotsPR, err := eots.RandGen(r)
		if err != nil {
			return nil, err
		}
		pr := bbn.NewSchnorrPubRandFromFieldVal(eotsPR)
		srList = append(srList, eotsSR)
		prList = append(prList, *pr)
	}

	prByteList := [][]byte{}
	for i := range prList {
		prByteList = append(prByteList, prList[i])
	}

	// generate the commitment to these public randomness
	commitment, proofList := merkle.ProofsFromByteSlices(prByteList)

	return &RandListInfo{srList, prList, commitment, proofList}, nil
}

func GenRandomMsgCommitPubRandList(
	r *rand.Rand,
	sk *btcec.PrivateKey,
	signingContext string,
	startHeight uint64,
	numPubRand uint64,
) (*RandListInfo, *ftypes.MsgCommitPubRandList, error) {
	randListInfo, err := GenRandomPubRandList(r, numPubRand)
	if err != nil {
		return nil, nil, err
	}

	msg := &ftypes.MsgCommitPubRandList{
		Signer:      GenRandomAccount().Address,
		FpBtcPk:     bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey()),
		StartHeight: startHeight,
		NumPubRand:  numPubRand,
		Commitment:  randListInfo.Commitment,
	}
	hash, err := msg.HashToSign(signingContext)
	if err != nil {
		return nil, nil, err
	}
	schnorrSig, err := schnorr.Sign(sk, hash)
	if err != nil {
		return nil, nil, err
	}
	msg.Sig = bbn.NewBIP340SignatureFromBTCSig(schnorrSig)
	return randListInfo, msg, nil
}

func NewMsgAddFinalitySig(
	signer string,
	sk *btcec.PrivateKey,
	signingContext string,
	startHeight uint64,
	blockHeight uint64,
	randListInfo *RandListInfo,
	blockAppHash []byte,
) (*ftypes.MsgAddFinalitySig, error) {
	idx := blockHeight - startHeight

	msg := &ftypes.MsgAddFinalitySig{
		Signer:       signer,
		FpBtcPk:      bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey()),
		PubRand:      &randListInfo.PRList[idx],
		Proof:        randListInfo.ProofList[idx].ToProto(),
		BlockHeight:  blockHeight,
		BlockAppHash: blockAppHash,
		FinalitySig:  nil,
	}
	msgToSign := msg.MsgToSign(signingContext)
	sig, err := eots.Sign(sk, randListInfo.SRList[idx], msgToSign)
	if err != nil {
		return nil, err
	}
	msg.FinalitySig = bbn.NewSchnorrEOTSSigFromModNScalar(sig)

	return msg, nil
}

func GenRandomEvidence(r *rand.Rand, sk *btcec.PrivateKey, height uint64) (*ftypes.Evidence, error) {
	pk := sk.PubKey()
	bip340PK := bbn.NewBIP340PubKeyFromBTCPK(pk)
	sr, pr, err := eots.RandGen(r)
	if err != nil {
		return nil, err
	}
	cAppHash := GenRandomByteArray(r, 32)
	cSig, err := eots.Sign(sk, sr, append(sdk.Uint64ToBigEndian(height), cAppHash...))
	if err != nil {
		return nil, err
	}
	fAppHash := GenRandomByteArray(r, 32)
	fSig, err := eots.Sign(sk, sr, append(sdk.Uint64ToBigEndian(height), fAppHash...))
	if err != nil {
		return nil, err
	}

	evidence := &ftypes.Evidence{
		FpBtcPk:              bip340PK,
		BlockHeight:          height,
		PubRand:              bbn.NewSchnorrPubRandFromFieldVal(pr),
		CanonicalAppHash:     cAppHash,
		ForkAppHash:          fAppHash,
		CanonicalFinalitySig: bbn.NewSchnorrEOTSSigFromModNScalar(cSig),
		ForkFinalitySig:      bbn.NewSchnorrEOTSSigFromModNScalar(fSig),
	}
	return evidence, nil
}

func GenRandomBlock(r *rand.Rand) *ftypes.IndexedBlock {
	return &ftypes.IndexedBlock{
		Height:  RandomInt(r, 1000000),
		AppHash: GenRandomByteArray(r, 32),
	}
}

func GenRandomBlockWithHeight(r *rand.Rand, height uint64) *ftypes.IndexedBlock {
	return &ftypes.IndexedBlock{
		Height:  height,
		AppHash: GenRandomByteArray(r, 32),
	}
}

func GenRandomFinalitySig(r *rand.Rand) (*bbn.SchnorrEOTSSig, error) {
	modNScalarBz := GenRandomByteArray(r, 32)
	var modNScalar btcec.ModNScalar
	if overflowed := modNScalar.SetByteSlice(modNScalarBz); overflowed {
		return nil, errors.New("modNScalar overflow")
	}
	return bbn.NewSchnorrEOTSSigFromModNScalar(&modNScalar), nil
}

func GenRandomFinalityGenesisState(r *rand.Rand, signingContext string) (*ftypes.GenesisState, error) {
	var (
		entriesCount   = int(RandomIntOtherThan(r, 0, 20)) + 1 // make sure there's always at least one entry
		blocks         = make([]*ftypes.IndexedBlock, entriesCount)
		evidences      = make([]*ftypes.Evidence, entriesCount)
		votes          = make([]*ftypes.VoteSig, entriesCount)
		pubRand        = make([]*ftypes.PublicRandomness, entriesCount)
		pubRandComm    = make([]*ftypes.PubRandCommitWithPK, entriesCount)
		pubRandCommIdx = make([]*ftypes.PubRandCommitIdx, entriesCount)
		signInfo       = make([]ftypes.SigningInfo, entriesCount)
		missedBlocks   = make([]ftypes.FinalityProviderMissedBlocks, entriesCount)
		votingPowers   = make([]*ftypes.VotingPowerFP, entriesCount)
		vpDistCache    = make([]*ftypes.VotingPowerDistCacheBlkHeight, entriesCount)
		params         = ftypes.DefaultParams()
	)

	for i := range entriesCount {
		// populate blocks
		blockHeight := uint64(i) + 1
		blocks[i] = GenRandomBlockWithHeight(r, blockHeight)

		// populate evidences
		sk, _, err := GenRandomBTCKeyPair(r)
		if err != nil {
			return nil, err
		}
		evidences[i], err = GenRandomEvidence(r, sk, blockHeight)
		if err != nil {
			return nil, err
		}

		// populate vots
		btcPk, err := GenRandomBIP340PubKey(r)
		if err != nil {
			return nil, err
		}
		fs, err := GenRandomFinalitySig(r)
		if err != nil {
			return nil, err
		}
		votes[i] = &ftypes.VoteSig{
			BlockHeight: blockHeight,
			FpBtcPk:     btcPk,
			FinalitySig: fs,
		}

		// populate pub rand
		prl, err := GenRandomPubRandList(r, 1)
		if err != nil {
			return nil, err
		}
		pubRand[i] = &ftypes.PublicRandomness{
			BlockHeight: blockHeight,
			FpBtcPk:     btcPk,
			PubRand:     &prl.PRList[0],
		}

		// populate pub rand commit
		pubRandComm[i] = &ftypes.PubRandCommitWithPK{
			FpBtcPk: btcPk,
			PubRandCommit: &ftypes.PubRandCommit{
				StartHeight: blockHeight,
				NumPubRand:  RandomInt(r, 10000),
				Commitment:  GenRandomByteArray(r, 32),
				EpochNum:    GenRandomEpochNum(r),
			},
		}

		pubRandCommIdx[i] = &ftypes.PubRandCommitIdx{
			FpBtcPk: btcPk,
			Index: &ftypes.PubRandCommitIndexValue{
				Heights: []uint64{blockHeight},
			},
		}

		// populate signing info
		signInfo[i] = ftypes.SigningInfo{
			FpBtcPk:       btcPk,
			FpSigningInfo: ftypes.NewFinalityProviderSigningInfo(btcPk, int64(blockHeight), int64(RandomInt(r, 10000))),
		}

		// populate missed blocks
		missedBlocks[i] = ftypes.FinalityProviderMissedBlocks{
			FpBtcPk: btcPk,
			MissedBlocks: []ftypes.MissedBlock{{
				Index:  int64(RandomInt(r, 10000)) % params.SignedBlocksWindow,
				Missed: true,
			}},
		}

		// populate voting powers
		votingPowers[i] = &ftypes.VotingPowerFP{
			BlockHeight: blockHeight,
			FpBtcPk:     btcPk,
			VotingPower: RandomInt(r, 100000),
		}

		// populate voting powers dist cache
		vpDistr, _, err := GenRandomVotingPowerDistCache(r, 100, signingContext)
		if err != nil {
			return nil, err
		}
		vpDistCache[i] = &ftypes.VotingPowerDistCacheBlkHeight{
			BlockHeight:    blockHeight,
			VpDistribution: vpDistr,
		}
	}

	return &ftypes.GenesisState{
		Params:               params,
		IndexedBlocks:        blocks,
		Evidences:            evidences,
		VoteSigs:             votes,
		PublicRandomness:     pubRand,
		PubRandCommit:        pubRandComm,
		SigningInfos:         signInfo,
		MissedBlocks:         missedBlocks,
		VotingPowers:         votingPowers,
		VpDstCache:           vpDistCache,
		NextHeightToFinalize: uint64(entriesCount),
		NextHeightToReward:   uint64(entriesCount),
		PubRandCommitIndexes: pubRandCommIdx,
	}, nil
}
