package babylon

import (
	"context"
	"encoding/json"
	"fmt"

	sdkErr "cosmossdk.io/errors"
	bbnclient "github.com/babylonlabs-io/babylon/client/client"
	"github.com/babylonlabs-io/babylon/client/config"
	"github.com/babylonlabs-io/babylon/test/e2e/clientcontroller/types"
	bbntypes "github.com/babylonlabs-io/babylon/types"
	btcstakingtypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	finalitytypes "github.com/babylonlabs-io/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

type BabylonConsumerController struct {
	bbnClient *bbnclient.Client
	cfg       *config.BabylonConfig
	logger    *zap.Logger
}

func NewBabylonConsumerController(
	cfg *config.BabylonConfig,
	logger *zap.Logger,
) (*BabylonConsumerController, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for Babylon client: %w", err)
	}

	bc, err := bbnclient.New(
		cfg,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Babylon client: %w", err)
	}

	return &BabylonConsumerController{
		bc,
		cfg,
		logger,
	}, nil
}

func (bc *BabylonConsumerController) mustGetTxSigner() string {
	signer := bc.GetKeyAddress()
	prefix := bc.cfg.AccountPrefix
	return sdk.MustBech32ifyAddressBytes(prefix, signer)
}

func (bc *BabylonConsumerController) GetKeyAddress() sdk.AccAddress {
	// get key address, retrieves address based on key name which is configured in
	// cfg *stakercfg.BBNConfig. If this fails, it means we have misconfiguration problem
	// and we should panic.
	// This is checked at the start of BabylonConsumerController, so if it fails something is really wrong

	keyRec, err := bc.bbnClient.GetKeyring().Key(bc.cfg.Key)

	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}

	addr, err := keyRec.GetAddress()

	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}

	return addr
}

func (bc *BabylonConsumerController) reliablySendMsg(msg sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return bc.reliablySendMsgs([]sdk.Msg{msg}, expectedErrs, unrecoverableErrs)
}

func (bc *BabylonConsumerController) reliablySendMsgs(msgs []sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return bc.bbnClient.ReliablySendMsgs(
		context.Background(),
		msgs,
		expectedErrs,
		unrecoverableErrs,
	)
}

// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
// it returns tx hash and error
func (bc *BabylonConsumerController) CommitPubRandList(
	fpPk *btcec.PublicKey,
	startHeight uint64,
	numPubRand uint64,
	commitment []byte,
	sig *schnorr.Signature,
) (*types.TxResponse, error) {
	msg := &finalitytypes.MsgCommitPubRandList{
		Signer:      bc.mustGetTxSigner(),
		FpBtcPk:     bbntypes.NewBIP340PubKeyFromBTCPK(fpPk),
		StartHeight: startHeight,
		NumPubRand:  numPubRand,
		Commitment:  commitment,
		Sig:         bbntypes.NewBIP340SignatureFromBTCSig(sig),
	}

	unrecoverableErrs := []*sdkErr.Error{
		finalitytypes.ErrInvalidPubRand,
		finalitytypes.ErrTooFewPubRand,
		finalitytypes.ErrNoPubRandYet,
		btcstakingtypes.ErrFpNotFound,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, unrecoverableErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash}, nil
}

// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
func (bc *BabylonConsumerController) SubmitFinalitySig(
	fpPk *btcec.PublicKey,
	block *types.BlockInfo,
	pubRand *btcec.FieldVal,
	proof []byte, // TODO: have a type for proof
	sig *btcec.ModNScalar,
) (*types.TxResponse, error) {
	cmtProof := cmtcrypto.Proof{}
	if err := cmtProof.Unmarshal(proof); err != nil {
		return nil, err
	}

	msg := &finalitytypes.MsgAddFinalitySig{
		Signer:       bc.mustGetTxSigner(),
		FpBtcPk:      bbntypes.NewBIP340PubKeyFromBTCPK(fpPk),
		BlockHeight:  block.Height,
		PubRand:      bbntypes.NewSchnorrPubRandFromFieldVal(pubRand),
		Proof:        &cmtProof,
		BlockAppHash: block.Hash,
		FinalitySig:  bbntypes.NewSchnorrEOTSSigFromModNScalar(sig),
	}

	unrecoverableErrs := []*sdkErr.Error{
		finalitytypes.ErrInvalidFinalitySig,
		finalitytypes.ErrPubRandNotFound,
		btcstakingtypes.ErrFpAlreadySlashed,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, unrecoverableErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash, Events: fromCosmosEventsToBytes(res.Events)}, nil
}

// SubmitBatchFinalitySigs submits a batch of finality signatures to Babylon
func (bc *BabylonConsumerController) SubmitBatchFinalitySigs(
	fpPk *btcec.PublicKey,
	blocks []*types.BlockInfo,
	pubRandList []*btcec.FieldVal,
	proofList [][]byte,
	sigs []*btcec.ModNScalar,
) (*types.TxResponse, error) {
	if len(blocks) != len(sigs) {
		return nil, fmt.Errorf("the number of blocks %v should match the number of finality signatures %v", len(blocks), len(sigs))
	}

	msgs := make([]sdk.Msg, 0, len(blocks))
	for i, b := range blocks {
		cmtProof := cmtcrypto.Proof{}
		if err := cmtProof.Unmarshal(proofList[i]); err != nil {
			return nil, err
		}

		msg := &finalitytypes.MsgAddFinalitySig{
			Signer:       bc.mustGetTxSigner(),
			FpBtcPk:      bbntypes.NewBIP340PubKeyFromBTCPK(fpPk),
			BlockHeight:  b.Height,
			PubRand:      bbntypes.NewSchnorrPubRandFromFieldVal(pubRandList[i]),
			Proof:        &cmtProof,
			BlockAppHash: b.Hash,
			FinalitySig:  bbntypes.NewSchnorrEOTSSigFromModNScalar(sigs[i]),
		}
		msgs = append(msgs, msg)
	}

	unrecoverableErrs := []*sdkErr.Error{
		finalitytypes.ErrInvalidFinalitySig,
		finalitytypes.ErrPubRandNotFound,
		btcstakingtypes.ErrFpAlreadySlashed,
	}

	res, err := bc.reliablySendMsgs(msgs, emptyErrs, unrecoverableErrs)
	if err != nil {
		return nil, err
	}

	return &types.TxResponse{TxHash: res.TxHash}, nil
}

// QueryFinalityProviderHasPower queries whether the finality provider has voting power at a given height
func (bc *BabylonConsumerController) QueryFinalityProviderHasPower(
	fpPk *btcec.PublicKey,
	blockHeight uint64,
) (bool, error) {
	res, err := bc.bbnClient.QueryClient.FinalityProviderPowerAtHeight(
		bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
		blockHeight,
	)
	if err != nil {
		return false, fmt.Errorf("failed to query the finality provider's voting power at height %d: %w", blockHeight, err)
	}

	return res.VotingPower > 0, nil
}

func (bc *BabylonConsumerController) QueryLatestFinalizedBlock() (*types.BlockInfo, error) {
	blocks, err := bc.queryLatestBlocks(nil, 1, finalitytypes.QueriedBlockStatus_FINALIZED, true)
	if blocks == nil {
		return nil, err
	}
	return blocks[0], err
}

func (bc *BabylonConsumerController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error) {
	if endHeight < startHeight {
		return nil, fmt.Errorf("the startHeight %v should not be higher than the endHeight %v", startHeight, endHeight)
	}
	count := endHeight - startHeight + 1
	if count > limit {
		count = limit
	}
	return bc.queryLatestBlocks(sdk.Uint64ToBigEndian(startHeight), count, finalitytypes.QueriedBlockStatus_ANY, false)
}

func (bc *BabylonConsumerController) queryLatestBlocks(startKey []byte, count uint64, status finalitytypes.QueriedBlockStatus, reverse bool) ([]*types.BlockInfo, error) {
	var blocks []*types.BlockInfo
	pagination := &sdkquery.PageRequest{
		Limit:   count,
		Reverse: reverse,
		Key:     startKey,
	}

	res, err := bc.bbnClient.QueryClient.ListBlocks(status, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to query finalized blocks: %v", err)
	}

	for _, b := range res.Blocks {
		ib := &types.BlockInfo{
			Height: b.Height,
			Hash:   b.AppHash,
		}
		blocks = append(blocks, ib)
	}

	return blocks, nil
}

func (bc *BabylonConsumerController) QueryBlock(height uint64) (*types.BlockInfo, error) {
	res, err := bc.bbnClient.QueryClient.Block(height)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexed block at height %v: %w", height, err)
	}

	return &types.BlockInfo{
		Height: height,
		Hash:   res.Block.AppHash,
	}, nil
}

// QueryLastPublicRandCommit returns the last public randomness commitments
func (bc *BabylonConsumerController) QueryLastPublicRandCommit(fpPk *btcec.PublicKey) (*types.PubRandCommit, error) {
	fpBtcPk := bbntypes.NewBIP340PubKeyFromBTCPK(fpPk)

	pagination := &sdkquery.PageRequest{
		Limit:   1,
		Reverse: true,
	}

	res, err := bc.bbnClient.QueryClient.ListPubRandCommit(fpBtcPk.MarshalHex(), pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to query committed public randomness: %w", err)
	}

	if len(res.PubRandCommitMap) == 0 {
		// expected when there is no PR commit at all
		return nil, nil
	}

	if len(res.PubRandCommitMap) > 1 {
		return nil, fmt.Errorf("expected length to be 1, but get :%d", len(res.PubRandCommitMap))
	}

	var commit *types.PubRandCommit = nil
	for height, commitRes := range res.PubRandCommitMap {
		commit = &types.PubRandCommit{
			StartHeight: height,
			NumPubRand:  commitRes.NumPubRand,
			Commitment:  commitRes.Commitment,
		}
	}

	if err := commit.Validate(); err != nil {
		return nil, err
	}

	return commit, nil
}

func (bc *BabylonConsumerController) QueryIsBlockFinalized(height uint64) (bool, error) {
	res, err := bc.bbnClient.QueryClient.Block(height)
	if err != nil {
		return false, fmt.Errorf("failed to query indexed block at height %v: %w", height, err)
	}

	return res.Block.Finalized, nil
}

func (bc *BabylonConsumerController) QueryActivatedHeight() (uint64, error) {
	res, err := bc.bbnClient.QueryClient.ActivatedHeight()
	if err != nil {
		return 0, fmt.Errorf("failed to query activated height: %w", err)
	}

	return res.Height, nil
}

func (bc *BabylonConsumerController) QueryLatestBlockHeight() (uint64, error) {
	blocks, err := bc.queryLatestBlocks(nil, 1, finalitytypes.QueriedBlockStatus_ANY, true)
	if err != nil || len(blocks) != 1 {
		// try query comet block if the index block query is not available
		block, err := bc.queryCometBestBlock()
		if err != nil {
			return 0, err
		}
		return block.Height, nil
	}

	return blocks[0].Height, nil
}

func (bc *BabylonConsumerController) queryCometBestBlock() (*types.BlockInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), bc.cfg.Timeout)
	// this will return 20 items at max in the descending order (highest first)
	chainInfo, err := bc.bbnClient.RPCClient.BlockchainInfo(ctx, 0, 0)
	defer cancel()

	if err != nil {
		return nil, err
	}

	// Returning response directly, if header with specified number did not exist
	// at request will contain nil header
	return &types.BlockInfo{
		Height: uint64(chainInfo.BlockMetas[0].Header.Height),
		Hash:   chainInfo.BlockMetas[0].Header.AppHash,
	}, nil
}

func (bc *BabylonConsumerController) Close() error {
	if !bc.bbnClient.IsRunning() {
		return nil
	}

	return bc.bbnClient.Stop()
}

func fromCosmosEventsToBytes(events []provider.RelayerEvent) []byte {
	bytes, err := json.Marshal(events)
	if err != nil {
		return nil
	}
	return bytes
}
