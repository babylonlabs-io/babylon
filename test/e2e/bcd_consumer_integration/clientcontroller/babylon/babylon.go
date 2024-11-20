// Package clientcontroller/babylon wraps the Babylon RPC/gRPC client for easy interaction with a Babylon node.
// It simplifies querying and submitting transactions.

// Core Babylon RPC/gRPC client lives under https://github.com/babylonlabs-io/babylon/tree/main/client/client

// Clientcontroller is adapted from:
// https://github.com/babylonlabs-io/finality-provider/blob/base/consumer-chain-support/clientcontroller/babylon/babylon.go

package babylon

import (
	"context"
	"fmt"
	"math/rand"

	sdkErr "cosmossdk.io/errors"
	"cosmossdk.io/math"
	bbnclient "github.com/babylonlabs-io/babylon/client/client"
	"github.com/babylonlabs-io/babylon/client/config"
	"github.com/babylonlabs-io/babylon/crypto/eots"
	types2 "github.com/babylonlabs-io/babylon/test/e2e/bcd_consumer_integration/clientcontroller/types"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbntypes "github.com/babylonlabs-io/babylon/types"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	btcstakingtypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	finalitytypes "github.com/babylonlabs-io/babylon/x/finality/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
	sttypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

var emptyErrs = []*sdkErr.Error{}

type BabylonController struct {
	bbnClient *bbnclient.Client
	cfg       *config.BabylonConfig
	btcParams *chaincfg.Params
	logger    *zap.Logger
}

func NewBabylonController(
	cfg *config.BabylonConfig,
	btcParams *chaincfg.Params,
	logger *zap.Logger,
) (*BabylonController, error) {
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

	return &BabylonController{
		bc,
		cfg,
		btcParams,
		logger,
	}, nil
}

func (bc *BabylonController) GetBBNClient() *bbnclient.Client {
	return bc.bbnClient
}

func (bc *BabylonController) MustGetTxSigner() string {
	signer := bc.GetKeyAddress()
	prefix := bc.cfg.AccountPrefix
	return sdk.MustBech32ifyAddressBytes(prefix, signer)
}

func (bc *BabylonController) GetKeyAddress() sdk.AccAddress {
	// get key address, retrieves address based on the key name which is configured in
	// cfg *stakercfg.BBNConfig. If this fails, it means we have a misconfiguration problem
	// and we should panic.
	// This is checked at the start of BabylonController, so if it fails something is really wrong

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

func (bc *BabylonController) reliablySendMsg(msg sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return bc.reliablySendMsgs([]sdk.Msg{msg}, expectedErrs, unrecoverableErrs)
}

func (bc *BabylonController) reliablySendMsgs(msgs []sdk.Msg, expectedErrs []*sdkErr.Error, unrecoverableErrs []*sdkErr.Error) (*provider.RelayerTxResponse, error) {
	return bc.bbnClient.ReliablySendMsgs(
		context.Background(),
		msgs,
		expectedErrs,
		unrecoverableErrs,
	)
}

// RegisterFinalityProvider registers a finality provider via a MsgCreateFinalityProvider to Babylon
// it returns tx hash and error
func (bc *BabylonController) RegisterFinalityProvider(
	chainID string,
	fpPk *bbntypes.BIP340PubKey,
	pop []byte,
	commission *math.LegacyDec,
	description []byte,
) (*types2.TxResponse, error) {
	var bbnPop btcstakingtypes.ProofOfPossessionBTC
	if err := bbnPop.Unmarshal(pop); err != nil {
		return nil, fmt.Errorf("invalid proof-of-possession: %w", err)
	}

	var sdkDescription sttypes.Description
	if err := sdkDescription.Unmarshal(description); err != nil {
		return nil, fmt.Errorf("invalid description: %w", err)
	}

	fpAddr := bc.MustGetTxSigner()
	msg := &btcstakingtypes.MsgCreateFinalityProvider{
		Addr:        fpAddr,
		BtcPk:       fpPk,
		Pop:         &bbnPop,
		Commission:  commission,
		Description: &sdkDescription,
		ConsumerId:  chainID,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types2.TxResponse{TxHash: res.TxHash}, nil
}

func (bc *BabylonController) QueryFinalityProviderSlashed(fpPk *btcec.PublicKey) (bool, error) {
	fpPubKey := bbntypes.NewBIP340PubKeyFromBTCPK(fpPk)
	res, err := bc.bbnClient.QueryClient.FinalityProvider(fpPubKey.MarshalHex())
	if err != nil {
		return false, fmt.Errorf("failed to query the finality provider %s: %v", fpPubKey.MarshalHex(), err)
	}

	slashed := res.FinalityProvider.SlashedBtcHeight > 0

	return slashed, nil
}

func (bc *BabylonController) QueryFinalityProvider(fpBtcPkHex string) (*btcstakingtypes.QueryFinalityProviderResponse, error) {
	res, err := bc.bbnClient.QueryClient.FinalityProvider(fpBtcPkHex)
	if err != nil {
		return nil, fmt.Errorf("failed to query the finality provider %s: %v", fpBtcPkHex, err)
	}

	return res, nil
}

func (bc *BabylonController) QueryNodeStatus() (*coretypes.ResultStatus, error) {
	return bc.bbnClient.QueryClient.GetStatus()
}

// QueryFinalityProviderHasPower queries whether the finality provider has voting power at a given height
func (bc *BabylonController) QueryFinalityProviderHasPower(fpPk *btcec.PublicKey, blockHeight uint64) (bool, error) {
	res, err := bc.bbnClient.QueryClient.FinalityProviderPowerAtHeight(
		bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
		blockHeight,
	)
	if err != nil {
		return false, fmt.Errorf("failed to query BTC delegations: %w", err)
	}

	return res.VotingPower > 0, nil
}

func (bc *BabylonController) QueryLatestFinalizedBlocks(count uint64) ([]*types2.BlockInfo, error) {
	return bc.queryLatestBlocks(nil, count, finalitytypes.QueriedBlockStatus_FINALIZED, true)
}

func (bc *BabylonController) QueryIndexedBlock(height uint64) (*finalitytypes.IndexedBlock, error) {
	resp, err := bc.bbnClient.Block(height)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexed block: %v", err)
	}

	return resp.Block, nil
}

func (bc *BabylonController) QueryCometBlock(height uint64) (*coretypes.ResultBlock, error) {
	return bc.bbnClient.GetBlock(int64(height))
}

func (bc *BabylonController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types2.BlockInfo, error) {
	if endHeight < startHeight {
		return nil, fmt.Errorf("the startHeight %v should not be higher than the endHeight %v", startHeight, endHeight)
	}
	count := endHeight - startHeight + 1
	if count > limit {
		count = limit
	}
	return bc.queryLatestBlocks(sdk.Uint64ToBigEndian(startHeight), count, finalitytypes.QueriedBlockStatus_ANY, false)
}

// QueryLastCommittedPublicRand returns the last public randomness commitments
func (bc *BabylonController) QueryLastCommittedPublicRand(fpPk *btcec.PublicKey, count uint64) (map[uint64]*finalitytypes.PubRandCommitResponse, error) {
	fpBtcPk := bbntypes.NewBIP340PubKeyFromBTCPK(fpPk)

	pagination := &sdkquery.PageRequest{
		// NOTE: the count is limited by pagination queries
		Limit:   count,
		Reverse: true,
	}

	res, err := bc.bbnClient.QueryClient.ListPubRandCommit(fpBtcPk.MarshalHex(), pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to query committed public randomness: %w", err)
	}

	return res.PubRandCommitMap, nil
}

func (bc *BabylonController) queryLatestBlocks(startKey []byte, count uint64, status finalitytypes.QueriedBlockStatus, reverse bool) ([]*types2.BlockInfo, error) {
	var blocks []*types2.BlockInfo
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
		ib := &types2.BlockInfo{
			Height: b.Height,
			Hash:   b.AppHash,
		}
		blocks = append(blocks, ib)
	}

	return blocks, nil
}

func (bc *BabylonController) Close() error {
	if !bc.bbnClient.IsRunning() {
		return nil
	}

	return bc.bbnClient.Stop()
}

/*
	Implementations for e2e tests only
*/

func (bc *BabylonController) CreateBTCDelegation(
	delBtcPk *bbntypes.BIP340PubKey,
	fpPks []*btcec.PublicKey,
	pop *btcstakingtypes.ProofOfPossessionBTC,
	stakingTime uint32,
	stakingValue int64,
	stakingTx []byte,
	stakingTxInclusionProof *btcstakingtypes.InclusionProof,
	slashingTx *btcstakingtypes.BTCSlashingTx,
	delSlashingSig *bbntypes.BIP340Signature,
	unbondingTx []byte,
	unbondingTime uint32,
	unbondingValue int64,
	unbondingSlashingTx *btcstakingtypes.BTCSlashingTx,
	delUnbondingSlashingSig *bbntypes.BIP340Signature,
) (*types2.TxResponse, error) {
	fpBtcPks := make([]bbntypes.BIP340PubKey, 0, len(fpPks))
	for _, v := range fpPks {
		fpBtcPks = append(fpBtcPks, *bbntypes.NewBIP340PubKeyFromBTCPK(v))
	}
	msg := &btcstakingtypes.MsgCreateBTCDelegation{
		StakerAddr:                    bc.MustGetTxSigner(),
		Pop:                           pop,
		BtcPk:                         delBtcPk,
		FpBtcPkList:                   fpBtcPks,
		StakingTime:                   stakingTime,
		StakingValue:                  stakingValue,
		StakingTx:                     stakingTx,
		StakingTxInclusionProof:       stakingTxInclusionProof,
		SlashingTx:                    slashingTx,
		DelegatorSlashingSig:          delSlashingSig,
		UnbondingTx:                   unbondingTx,
		UnbondingTime:                 unbondingTime,
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           unbondingSlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSlashingSig,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types2.TxResponse{TxHash: res.TxHash}, nil
}

func (bc *BabylonController) InsertWBTCHeaders(r *rand.Rand) error {
	params, err := bc.QueryStakingParams()
	if err != nil {
		return fmt.Errorf("failed to query staking params: %w", err)
	}

	btcTipResp, err := bc.QueryBtcLightClientTip()
	if err != nil {
		return fmt.Errorf("failed to query BTC light client tip: %w", err)
	}

	tipHeader, err := bbntypes.NewBTCHeaderBytesFromHex(btcTipResp.HeaderHex)
	if err != nil {
		return fmt.Errorf("failed to create BTC header from hex: %w", err)
	}

	wHeaders := datagen.NewBTCHeaderChainFromParentInfo(r, &btclctypes.BTCHeaderInfo{
		Header: &tipHeader,
		Hash:   tipHeader.Hash(),
		Height: btcTipResp.Height,
		Work:   &btcTipResp.Work,
	}, uint32(params.FinalizationTimeoutBlocks))

	_, err = bc.InsertBtcBlockHeaders(wHeaders.ChainToBytes())
	if err != nil {
		return fmt.Errorf("failed to insert BTC block headers: %w", err)
	}

	return nil
}

func (bc *BabylonController) InsertBtcBlockHeaders(headers []bbntypes.BTCHeaderBytes) (*provider.RelayerTxResponse, error) {
	msg := &btclctypes.MsgInsertHeaders{
		Signer:  bc.MustGetTxSigner(),
		Headers: headers,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// TODO: only used in test. this should not be put here. it causes confusion that this is a method
// that will be used when FP runs. in that's the case, it implies it should work all all consumer
// types. but `bbnClient.QueryClient.FinalityProviders` doesn't work for consumer chains
func (bc *BabylonController) QueryFinalityProviders() ([]*btcstakingtypes.FinalityProviderResponse, error) {
	var fps []*btcstakingtypes.FinalityProviderResponse
	pagination := &sdkquery.PageRequest{
		Limit: 100,
	}

	for {
		res, err := bc.bbnClient.QueryClient.FinalityProviders(pagination)
		if err != nil {
			return nil, fmt.Errorf("failed to query finality providers: %v", err)
		}
		fps = append(fps, res.FinalityProviders...)
		if res.Pagination == nil || res.Pagination.NextKey == nil {
			break
		}

		pagination.Key = res.Pagination.NextKey
	}

	return fps, nil
}

func (bc *BabylonController) QueryConsumerFinalityProviders(consumerId string) ([]*bsctypes.FinalityProviderResponse, error) {
	var fps []*bsctypes.FinalityProviderResponse
	pagination := &sdkquery.PageRequest{
		Limit: 100,
	}

	for {
		res, err := bc.bbnClient.QueryClient.QueryConsumerFinalityProviders(consumerId, pagination)
		if err != nil {
			return nil, fmt.Errorf("failed to query finality providers: %v", err)
		}
		fps = append(fps, res.FinalityProviders...)
		if res.Pagination == nil || res.Pagination.NextKey == nil {
			break
		}

		pagination.Key = res.Pagination.NextKey
	}

	return fps, nil
}

func (bc *BabylonController) QueryConsumerFinalityProvider(consumerId, fpBtcPkHex string) (*bsctypes.FinalityProviderResponse, error) {
	res, err := bc.bbnClient.QueryClient.QueryConsumerFinalityProvider(consumerId, fpBtcPkHex)
	if err != nil {
		return nil, fmt.Errorf("failed to query finality provider: %v", err)
	}

	return res, nil
}

func (bc *BabylonController) QueryBtcLightClientTip() (*btclctypes.BTCHeaderInfoResponse, error) {
	res, err := bc.bbnClient.QueryClient.BTCHeaderChainTip()
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC tip: %v", err)
	}

	return res.Header, nil
}

// TODO: this method only used in test. this should be refactored out to test files
func (bc *BabylonController) QueryVotesAtHeight(height uint64) ([]bbntypes.BIP340PubKey, error) {
	res, err := bc.bbnClient.QueryClient.VotesAtHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC delegations: %w", err)
	}

	return res.BtcPks, nil
}

func (bc *BabylonController) QueryPendingDelegations(limit uint64) ([]*btcstakingtypes.BTCDelegationResponse, error) {
	return bc.queryDelegationsWithStatus(btcstakingtypes.BTCDelegationStatus_PENDING, limit)
}

func (bc *BabylonController) QueryActiveDelegations(limit uint64) ([]*btcstakingtypes.BTCDelegationResponse, error) {
	return bc.queryDelegationsWithStatus(btcstakingtypes.BTCDelegationStatus_ACTIVE, limit)
}

func (bc *BabylonController) QueryBTCDelegation(stakingTxHashHex string) (*btcstakingtypes.BTCDelegationResponse, error) {
	resp, err := bc.bbnClient.QueryClient.BTCDelegation(stakingTxHashHex)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC delegation: %w", err)
	}

	if resp.BtcDelegation == nil {
		return nil, fmt.Errorf("no BTC delegation found for staking tx hash: %s", stakingTxHashHex)
	}

	return resp.BtcDelegation, nil
}

func (bc *BabylonController) QueryFinalityProviderDelegations(fpBtcPkHex string, limit uint64) ([]*btcstakingtypes.BTCDelegatorDelegationsResponse, error) {
	pagination := &sdkquery.PageRequest{
		Limit: limit,
	}

	resp, err := bc.bbnClient.QueryClient.FinalityProviderDelegations(fpBtcPkHex, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to query finality provider delegations: %w", err)
	}

	return resp.BtcDelegatorDelegations, nil
}

func (bc *BabylonController) QueryActivatedHeight() (*finalitytypes.QueryActivatedHeightResponse, error) {
	resp, err := bc.bbnClient.QueryClient.ActivatedHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to query activated height: %w", err)
	}

	return resp, nil
}

// queryDelegationsWithStatus queries BTC delegations
// with the given status (either pending or unbonding)
// it is only used when the program is running in Covenant mode
func (bc *BabylonController) queryDelegationsWithStatus(status btcstakingtypes.BTCDelegationStatus, limit uint64) ([]*btcstakingtypes.BTCDelegationResponse, error) {
	pagination := &sdkquery.PageRequest{
		Limit: limit,
	}

	res, err := bc.bbnClient.QueryClient.BTCDelegations(status, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC delegations: %v", err)
	}

	return res.BtcDelegations, nil
}

func (bc *BabylonController) QueryStakingParams() (*types2.StakingParams, error) {
	// query btc checkpoint params
	ckptParamRes, err := bc.bbnClient.QueryClient.BTCCheckpointParams()
	if err != nil {
		return nil, fmt.Errorf("failed to query params of the btccheckpoint module: %v", err)
	}

	// query btc staking params
	stakingParamRes, err := bc.bbnClient.QueryClient.BTCStakingParams()
	if err != nil {
		return nil, fmt.Errorf("failed to query staking params: %v", err)
	}

	covenantPks := make([]*btcec.PublicKey, 0, len(stakingParamRes.Params.CovenantPks))
	for _, pk := range stakingParamRes.Params.CovenantPks {
		covPk, err := pk.ToBTCPK()
		if err != nil {
			return nil, fmt.Errorf("invalid covenant public key")
		}
		covenantPks = append(covenantPks, covPk)
	}

	return &types2.StakingParams{
		ComfirmationTimeBlocks:    ckptParamRes.Params.BtcConfirmationDepth,
		FinalizationTimeoutBlocks: ckptParamRes.Params.CheckpointFinalizationTimeout,
		MinSlashingTxFeeSat:       btcutil.Amount(stakingParamRes.Params.MinSlashingTxFeeSat),
		CovenantPks:               covenantPks,
		SlashingPkScript:          stakingParamRes.Params.SlashingPkScript,
		CovenantQuorum:            stakingParamRes.Params.CovenantQuorum,
		SlashingRate:              stakingParamRes.Params.SlashingRate,
		MinUnbondingTime:          stakingParamRes.Params.MinUnbondingTimeBlocks,
	}, nil
}

func (bc *BabylonController) QueryBTCStakingParams() (*btcstakingtypes.Params, error) {
	res, err := bc.bbnClient.QueryClient.BTCStakingParams()
	if err != nil {
		return nil, fmt.Errorf("failed to query BTC staking params: %v", err)
	}

	return &res.Params, nil
}

func (bc *BabylonController) SubmitCovenantSigs(
	covPk *btcec.PublicKey,
	stakingTxHash string,
	slashingSigs [][]byte,
	unbondingSig *schnorr.Signature,
	unbondingSlashingSigs [][]byte,
) (*types2.TxResponse, error) {
	bip340UnbondingSig := bbntypes.NewBIP340SignatureFromBTCSig(unbondingSig)

	msg := &btcstakingtypes.MsgAddCovenantSigs{
		Signer:                  bc.MustGetTxSigner(),
		Pk:                      bbntypes.NewBIP340PubKeyFromBTCPK(covPk),
		StakingTxHash:           stakingTxHash,
		SlashingTxSigs:          slashingSigs,
		UnbondingTxSig:          bip340UnbondingSig,
		SlashingUnbondingTxSigs: unbondingSlashingSigs,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types2.TxResponse{TxHash: res.TxHash}, nil
}

func (bc *BabylonController) InsertSpvProofs(submitter string, proofs []*btcctypes.BTCSpvProof) (*provider.RelayerTxResponse, error) {
	msg := &btcctypes.MsgInsertBTCSpvProof{
		Submitter: submitter,
		Proofs:    proofs,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// RegisterConsumerChain registers a consumer chain via a MsgRegisterChain to Babylon
func (bc *BabylonController) RegisterConsumerChain(id, name, description string) (*types2.TxResponse, error) {
	msg := &bsctypes.MsgRegisterConsumer{
		Signer:              bc.MustGetTxSigner(),
		ConsumerId:          id,
		ConsumerName:        name,
		ConsumerDescription: description,
	}

	res, err := bc.reliablySendMsg(msg, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types2.TxResponse{TxHash: res.TxHash}, nil
}

func (bc *BabylonController) CommitPublicRandomness(
	msgCommitPubRandList *finalitytypes.MsgCommitPubRandList,
) (*types2.TxResponse, error) {
	signerAddr := bc.MustGetTxSigner()
	msgCommitPubRandList.Signer = signerAddr
	res, err := bc.reliablySendMsg(msgCommitPubRandList, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}

	return &types2.TxResponse{TxHash: res.TxHash}, nil
}

func (bc *BabylonController) SubmitFinalitySignature(
	fpSK *btcec.PrivateKey,
	fpBtcPk *bbntypes.BIP340PubKey,
	privateRand *eots.PrivateRand,
	pubRand *bbntypes.SchnorrPubRand,
	proof *cmtcrypto.Proof,
	heightToVote uint64,
) (*types2.TxResponse, error) {
	block, err := bc.bbnClient.QueryClient.GetBlock(int64(heightToVote))
	if err != nil {
		return nil, err
	}
	msgToSign := append(sdk.Uint64ToBigEndian(heightToVote), block.Block.AppHash...)
	sig, err := eots.Sign(fpSK, privateRand, msgToSign)
	if err != nil {
		return nil, err
	}
	eotsSig := bbntypes.NewSchnorrEOTSSigFromModNScalar(sig)

	signerAddr := bc.MustGetTxSigner()

	msgAddFinalitySig := &finalitytypes.MsgAddFinalitySig{
		Signer:       signerAddr,
		FpBtcPk:      fpBtcPk,
		BlockHeight:  heightToVote,
		PubRand:      pubRand,
		Proof:        proof,
		BlockAppHash: block.Block.AppHash,
		FinalitySig:  eotsSig,
	}
	res, err := bc.reliablySendMsg(msgAddFinalitySig, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}
	return &types2.TxResponse{TxHash: res.TxHash}, nil
}

func (bc *BabylonController) SubmitInvalidFinalitySignature(
	r *rand.Rand,
	fpSK *btcec.PrivateKey,
	fpBtcPk *bbntypes.BIP340PubKey,
	privateRand *eots.PrivateRand,
	pubRand *bbntypes.SchnorrPubRand,
	proof *cmtcrypto.Proof,
	heightToVote uint64,
) (*types2.TxResponse, error) {
	invalidAppHash := datagen.GenRandomByteArray(r, 32)
	invalidMsgToSign := append(sdk.Uint64ToBigEndian(heightToVote), invalidAppHash...)
	invalidSig, err := eots.Sign(fpSK, privateRand, invalidMsgToSign)
	if err != nil {
		return nil, err
	}
	invalidEotsSig := bbntypes.NewSchnorrEOTSSigFromModNScalar(invalidSig)

	signerAddr := bc.MustGetTxSigner()

	msgAddFinalitySig := &finalitytypes.MsgAddFinalitySig{
		Signer:       signerAddr,
		FpBtcPk:      fpBtcPk,
		BlockHeight:  heightToVote,
		PubRand:      pubRand,
		Proof:        proof,
		BlockAppHash: invalidAppHash,
		FinalitySig:  invalidEotsSig,
	}
	res, err := bc.reliablySendMsg(msgAddFinalitySig, emptyErrs, emptyErrs)
	if err != nil {
		return nil, err
	}
	return &types2.TxResponse{TxHash: res.TxHash}, nil
}

// IBCChannels queries the IBC channels
func (bc *BabylonController) IBCChannels() (*channeltypes.QueryChannelsResponse, error) {
	return bc.bbnClient.IBCChannels()
}

func (bc *BabylonController) QueryConsumerRegistry(consumerID string) (*bsctypes.QueryConsumersRegistryResponse, error) {
	return bc.bbnClient.QueryConsumersRegistry([]string{consumerID})
}

func (bc *BabylonController) QueryChannelClientState(channelID, portID string) (*channeltypes.QueryChannelClientStateResponse, error) {
	var resp *channeltypes.QueryChannelClientStateResponse
	err := bc.bbnClient.QueryClient.QueryIBCChannel(func(ctx context.Context, queryClient channeltypes.QueryClient) error {
		var err error
		req := &channeltypes.QueryChannelClientStateRequest{
			ChannelId: channelID,
			PortId:    portID,
		}
		resp, err = queryClient.ChannelClientState(ctx, req)
		return err
	})

	return resp, err
}
