package tmanager

import (
	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/crypto/eots"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	crypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// FinalityProvider represents a finality provider actor
type FinalityProvider struct {
	*WalletSender
	BtcPrivKey *btcec.PrivateKey
	PublicKey  *bbn.BIP340PubKey
	Commission math.LegacyDec

	PubRandListIdx       uint64
	CommitStartHeight    uint64
	PubRandList          *datagen.RandListInfo
	LastBlockHeightVoted uint64
}

func (n *Node) NewFpWithWallet(wallet *WalletSender) *FinalityProvider {
	fpSK, _, err := datagen.GenRandomBTCKeyPair(n.Tm.R)
	require.NoError(n.T(), err)

	fp, err := datagen.GenCustomFinalityProvider(n.Tm.R, fpSK, wallet.Address)
	require.NoError(n.T(), err)

	n.CreateFinalityProvider(wallet.KeyName, fp)
	n.WaitForNextBlock()

	fpResp := n.QueryFinalityProvider(fp.BtcPk.MarshalHex())
	require.NotNil(n.T(), fpResp)

	return &FinalityProvider{
		WalletSender: wallet,
		BtcPrivKey:   fpSK,
		PublicKey:    fp.BtcPk,
		Commission:   *fp.Commission,
	}
}

func (fp *FinalityProvider) CommitPubRand() {
	numPubRand := uint64(1000)
	commitStartHeight := uint64(5)

	randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(fp.Node.Tm.R, fp.BtcPrivKey, commitStartHeight, numPubRand)
	require.NoError(fp.T(), err)
	msgCommitPubRandList.Signer = fp.Addr()

	_, tx := fp.SubmitMsgs(msgCommitPubRandList)
	require.NotNil(fp.T(), tx, "FP CommitPubRandList transaction should not be nil")
	fp.T().Logf("Commited FP pub rand list: %s", fp.PublicKey.MarshalHex())

	fp.PubRandList = randListInfo
	fp.CommitStartHeight = commitStartHeight
}

func (fp *FinalityProvider) AddFinalityVoteUntilCurrentHeight() {
	blkNum, err := fp.Node.LatestBlockNumber()
	require.NoError(fp.T(), err)

	verifyTx := fp.WalletSender.VerifySentTx
	fp.WalletSender.VerifySentTx = false
	for fp.LastBlockHeightVoted < blkNum {
		fp.AddFinalityVote()
	}
	fp.WalletSender.VerifySentTx = verifyTx
}

func (fp *FinalityProvider) AddFinalityVote() {
	blockHeightToVote := fp.LastBlockHeightVoted + 1
	if fp.LastBlockHeightVoted == 0 {
		blockHeightToVote = fp.Node.QueryActivatedHeight()
	}

	pubRandIdx := fp.PubRandListIdx
	if fp.PubRandListIdx == 0 {
		pubRandIdx = blockHeightToVote - fp.CommitStartHeight
	}

	blockToVote := fp.Node.QueryBlock(int64(blockHeightToVote))
	appHash := blockToVote.AppHash

	msgToSign := append(sdk.Uint64ToBigEndian(blockHeightToVote), appHash...)
	// generate EOTS signature
	sig, err := eots.Sign(fp.BtcPrivKey, fp.PubRandList.SRList[pubRandIdx], msgToSign)
	require.NoError(fp.T(), err)

	finalitySig := bbn.NewSchnorrEOTSSigFromModNScalar(sig)

	proof := fp.PubRandList.ProofList[pubRandIdx]
	fp.AddFinalitySig(
		blockHeightToVote,
		&fp.PubRandList.PRList[pubRandIdx],
		proof.ToProto(),
		appHash,
		*finalitySig,
	)

	fp.LastBlockHeightVoted = blockHeightToVote
	fp.PubRandListIdx = pubRandIdx + 1
}

func (fp *FinalityProvider) AddFinalitySig(
	blockHeight uint64,
	pubRand *bbn.SchnorrPubRand,
	proof *crypto.Proof,
	blockAppHash []byte,
	finalitySig bbn.SchnorrEOTSSig,
) {
	msg := &ftypes.MsgAddFinalitySig{
		Signer:       fp.Addr(),
		FpBtcPk:      fp.PublicKey,
		BlockHeight:  blockHeight,
		PubRand:      pubRand,
		Proof:        proof,
		BlockAppHash: blockAppHash,
		FinalitySig:  &finalitySig,
	}

	_, tx := fp.SubmitMsgs(msg)
	require.NotNil(fp.T(), tx, "FP AddFinalitySig transaction should not be nil")
	fp.T().Logf("Added finality sig vote for height: %d", blockHeight)
}
