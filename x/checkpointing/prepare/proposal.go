package prepare

import (
	"bytes"
	"encoding/hex"
	"fmt"

	cometproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/codec/unknownproto"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	ckpttypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
)

const (
	defaultInjectedTxIndex = 0
	// MaxVoteExtensionSize is the maximum allowed size for a vote extension.
	// Breakdown of legitimate VoteExtension fields (~228 bytes):
	//   - Signer (bech32 address):       ~67 bytes
	//   - ValidatorAddress (bech32):     ~67 bytes
	//   - BlockHash (SHA-256):            34 bytes
	//   - EpochNum (varint):              ~4 bytes
	//   - Height (varint):                ~6 bytes
	//   - BlsSig (BLS12-381 signature):   50 bytes
	// Setting to 1KB provides ~4x overhead
	// while preventing memory amplification attacks (100 validators × 1KB = 100KB per block).
	MaxVoteExtensionSize = 1024 // 1KB
)

type SigValidationFn func(ctx sdk.Context, extendedVotes *abci.ExtendedCommitInfo, blockHash []byte) []ckpttypes.BlsSig

var (
	EmptyProposalRes = abci.ResponsePrepareProposal{Txs: [][]byte{}}
)

type ProposalHandler struct {
	logger     log.Logger
	ckptKeeper CheckpointingKeeper
	bApp       *baseapp.BaseApp

	// used for building and parsing the injected tx
	txConfig          client.TxConfig
	interfaceRegistry codectypes.InterfaceRegistry
	mp                mempool.Mempool

	defaultPrepareProposalHandler sdk.PrepareProposalHandler
	defaultProcessProposalHandler sdk.ProcessProposalHandler
}

func NewProposalHandler(
	logger log.Logger,
	ckptKeeper CheckpointingKeeper,
	mp mempool.Mempool,
	bApp *baseapp.BaseApp,
	encCfg *appparams.EncodingConfig,
) *ProposalHandler {
	defaultHandler := baseapp.NewDefaultProposalHandler(mp, bApp)
	ckpttypes.RegisterInterfaces(encCfg.InterfaceRegistry)

	return &ProposalHandler{
		logger:                        logger,
		mp:                            mp,
		ckptKeeper:                    ckptKeeper,
		bApp:                          bApp,
		txConfig:                      encCfg.TxConfig,
		interfaceRegistry:             encCfg.InterfaceRegistry,
		defaultPrepareProposalHandler: defaultHandler.PrepareProposalHandler(),
		defaultProcessProposalHandler: defaultHandler.ProcessProposalHandler(),
	}
}

func (h *ProposalHandler) SetHandlers(bApp *baseapp.BaseApp) {
	bApp.SetPrepareProposal(h.PrepareProposal())
	bApp.SetProcessProposal(h.ProcessProposal())
}

// PrepareProposal examines the vote extensions from the previous block, accumulates
// them into a checkpoint, and injects the checkpoint into the current proposal
// as a special tx
// Warning: the returned error of the handler will cause panic of the proposer,
// therefore we only return error when something really wrong happened
func (h *ProposalHandler) PrepareProposal() sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		// call default handler first to do basic validation
		res, err := h.defaultPrepareProposalHandler(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed in default PrepareProposal handler: %w", err)
		}

		k := h.ckptKeeper
		defaultProposalRes := &abci.ResponsePrepareProposal{Txs: res.Txs}

		epoch := k.GetEpoch(ctx)
		// BLS signatures are sent in the last block of the previous epoch,
		// so they should be aggregated in the first block of the new epoch
		// and no BLS signatures are send in epoch 0
		if !epoch.IsVoteExtensionProposal(ctx) {
			return defaultProposalRes, nil
		}

		proposalTxs, err := NewPrepareProposalTxs(req)
		if err != nil {
			return &EmptyProposalRes, fmt.Errorf("NewPrepareProposalTxs error: %w", err)
		}

		if len(req.LocalLastCommit.Votes) == 0 {
			return &EmptyProposalRes, fmt.Errorf("no extended votes received from the last block")
		}

		// 1. verify the validity of vote extensions (2/3 majority is achieved)
		err = baseapp.ValidateVoteExtensions(ctx, h.ckptKeeper, req.Height, ctx.ChainID(), req.LocalLastCommit)
		if err != nil {
			return &EmptyProposalRes, fmt.Errorf("invalid vote extensions: %w", err)
		}

		// 2. build a checkpoint for the previous epoch
		// Note: the epoch has not increased yet, so
		// we can use the current epoch
		ckpt, err := h.buildCheckpointFromVoteExtensions(
			ctx,
			epoch.EpochNumber,
			&req.LocalLastCommit,
			h.getValidBlsSigsAndPruneCommitInfo,
		)
		if err != nil {
			return &EmptyProposalRes, fmt.Errorf("failed to build checkpoint from vote extensions: %w", err)
		}

		// 3. inject a checkpoint tx into the proposal s.t. validators can decode, verify the checkpoint
		injectedVoteExtTx, err := h.buildInjectedTxBytes(ckpt, &req.LocalLastCommit)
		if err != nil {
			return &EmptyProposalRes, fmt.Errorf("failed to encode vote extensions into a special tx: %w", err)
		}

		err = proposalTxs.SetOrReplaceCheckpointTx(injectedVoteExtTx)
		if err != nil {
			return &EmptyProposalRes, fmt.Errorf("failed to inject checkpoint tx into the proposal: %w", err)
		}

		err = proposalTxs.ReplaceOtherTxs(res.Txs)
		if err != nil {
			return &EmptyProposalRes, fmt.Errorf("failed to add other txs into the proposal: %w", err)
		}

		return &abci.ResponsePrepareProposal{
			Txs: proposalTxs.GetTxsInOrder(),
		}, nil
	}
}

// buildCheckpointFromVoteExtensions builds a checkpoint from vote extensions. If
// pruneInvalidVotes is true, it will prune invalid votes from the provided
// commit info.
func (h *ProposalHandler) buildCheckpointFromVoteExtensions(
	ctx sdk.Context,
	epoch uint64,
	extCommit *abci.ExtendedCommitInfo,
	sigValidationFn SigValidationFn,
) (*ckpttypes.RawCheckpointWithMeta, error) {
	prevBlockID, err := h.findLastBlockHash(extCommit.Votes)
	if err != nil {
		return nil, err
	}
	ckpt := ckpttypes.NewCheckpointWithMeta(
		ckpttypes.NewCheckpoint(epoch, prevBlockID),
		ckpttypes.Accumulating,
	)
	validBLSSigs := sigValidationFn(
		ctx,
		extCommit,
		prevBlockID,
	)
	vals := h.ckptKeeper.GetValidatorSet(ctx, epoch)
	totalPower := h.ckptKeeper.GetTotalVotingPower(ctx, epoch)

	for _, sig := range validBLSSigs {
		signerAddress, err := sdk.ValAddressFromBech32(sig.SignerAddress)
		if err != nil {
			h.logger.Error(
				"skip invalid BLS sig",
				"invalid signer address", sig.SignerAddress,
				"err", err,
			)
			continue
		}
		signerBlsKey, err := h.ckptKeeper.GetBlsPubKey(ctx, signerAddress)
		if err != nil {
			h.logger.Error(
				"skip invalid BLS sig",
				"can't find BLS public key", err,
			)
			continue
		}
		err = ckpt.Accumulate(vals, signerAddress, signerBlsKey, *sig.BlsSig, totalPower)
		if err != nil {
			h.logger.Error(
				"skip invalid BLS sig",
				"accumulation failed", err,
			)
			continue
		}
		// sufficient voting power is accumulated
		if ckpt.Status == ckpttypes.Sealed {
			break
		}
	}
	if ckpt.Status != ckpttypes.Sealed {
		return nil, fmt.Errorf("insufficient voting power to build the checkpoint")
	}

	return ckpt, nil
}

func (h *ProposalHandler) VerifyVoteExtension(
	ctx sdk.Context,
	veBytes []byte,
	expectedBlockHash []byte,
) (*ckpttypes.BlsSig, error) {
	if len(veBytes) == 0 {
		return nil, fmt.Errorf("vote extension is empty")
	}

	var ve ckpttypes.VoteExtension
	if err := unknownproto.RejectUnknownFieldsStrict(veBytes, &ve, h.interfaceRegistry); err != nil {
		return nil, fmt.Errorf("vote extension contains unknown or extra bytes: %w", err)
	}

	if err := ve.Unmarshal(veBytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal vote extension: %w", err)
	}

	// check overall size and tries to marshal and unmarshal the same structure to check
	// for hidden fields sent to the vote extension
	veBytesLen := len(veBytes)
	if veBytesLen > MaxVoteExtensionSize {
		return nil, ckpttypes.ErrVoteExt.Wrapf("max size: %d, vote ext size: %d", MaxVoteExtensionSize, veBytesLen)
	}

	bzVoteExtAfterParse, err := ve.Marshal()
	if err != nil {
		return nil, ckpttypes.ErrVoteExt.Wrapf("failed to marshal vote ext seccond time: %s", err.Error())
	}

	if !bytes.Equal(veBytes, bzVoteExtAfterParse) {
		return nil, ckpttypes.ErrVoteExt.Wrapf(
			"malformed vote extension (possible malicious bytes included): original size %d, size after marshal %d",
			veBytesLen, len(bzVoteExtAfterParse),
		)
	}

	if err := ve.Validate(); err != nil {
		return nil, fmt.Errorf("invalid vote extension: %w", err)
	}

	_, err = sdk.ValAddressFromBech32(ve.Signer)
	if err != nil {
		return nil, fmt.Errorf("invalid signer address in vote extension: %w", err)
	}

	_, err = sdk.ValAddressFromBech32(ve.ValidatorAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid validator address in vote extension: %w", err)
	}

	if !bytes.Equal(*ve.BlockHash, expectedBlockHash) {
		return nil, fmt.Errorf("the BLS sig is signed over unexpected block hash. Expected: %s, Got: %s",
			hex.EncodeToString(expectedBlockHash), ve.BlockHash.String())
	}

	sig := ve.ToBLSSig()
	if err := h.ckptKeeper.VerifyBLSSig(ctx, sig); err != nil {
		return nil, fmt.Errorf("invalid BLS signature: %w", err)
	}

	return sig, nil
}

func (h *ProposalHandler) getValidBlsSigs(
	ctx sdk.Context,
	extendedVotes *abci.ExtendedCommitInfo,
	blockHash []byte,
) []ckpttypes.BlsSig {
	var validBLSSigs []ckpttypes.BlsSig

	for _, vote := range extendedVotes.Votes {
		sig, err := h.VerifyVoteExtension(ctx, vote.VoteExtension, blockHash)
		if err != nil {
			h.logger.Error("invalid vote extension", "err", err)
			continue
		}
		validBLSSigs = append(validBLSSigs, *sig)
	}

	return validBLSSigs
}

func (h *ProposalHandler) getValidBlsSigsAndPruneCommitInfo(
	ctx sdk.Context,
	extendedVotes *abci.ExtendedCommitInfo,
	blockHash []byte,
) []ckpttypes.BlsSig {
	var validBLSSigs []ckpttypes.BlsSig

	for i, vote := range extendedVotes.Votes {
		sig, err := h.VerifyVoteExtension(ctx, vote.VoteExtension, blockHash)

		if err != nil {
			h.logger.Error("invalid vote extension", "err", err)

			// We are marking votes with invalid vote extensions as absent
			vote.BlockIdFlag = cometproto.BlockIDFlagAbsent
			vote.ExtensionSignature = nil
			vote.VoteExtension = nil
			extendedVotes.Votes[i] = vote

			continue
		}

		validBLSSigs = append(validBLSSigs, *sig)
	}

	return validBLSSigs
}

// findLastBlockHash iterates over all vote extensions and finds the block hash
// that consensus has agreed upon.
// We need to iterate over all block hashes to find the one that has achieved consensus
// as CometBFT does not verify vote extensions once it has achieved >2/3 voting power in a block.
// Contract: This function should only be called for Vote Extensions
// that have been included in a previous block.
func (h *ProposalHandler) findLastBlockHash(extendedVotes []abci.ExtendedVoteInfo) ([]byte, error) {
	// Mapping between block hashes and voting power committed to them
	blockHashes := make(map[string]int64, 0)
	// Iterate over vote extensions and if they have a valid structure
	// increase the voting power of the block hash they commit to
	var totalPower int64 = 0
	for _, vote := range extendedVotes {
		// accumulate voting power from all the votes
		totalPower += vote.Validator.Power
		var ve ckpttypes.VoteExtension
		if len(vote.VoteExtension) == 0 {
			continue
		}
		if err := ve.Unmarshal(vote.VoteExtension); err != nil {
			continue
		}
		if ve.BlockHash == nil {
			continue
		}
		bHash, err := ve.BlockHash.Marshal()
		if err != nil {
			continue
		}
		// Encode the block hash using hex
		blockHashes[hex.EncodeToString(bHash)] += vote.Validator.Power
	}
	var (
		maxPower     int64 = 0
		resBlockHash string
	)
	// Find the block hash that has the maximum voting power committed to it
	for blockHash, power := range blockHashes {
		if power > maxPower {
			resBlockHash = blockHash
			maxPower = power
		}
	}
	if len(resBlockHash) == 0 {
		return nil, fmt.Errorf("could not find the block hash")
	}
	// Verify that the voting power committed to the found block hash is
	// more than 2/3 of the total voting power.
	if requiredVP := ((totalPower * 2) / 3) + 1; maxPower < requiredVP {
		return nil, fmt.Errorf(
			"insufficient cumulative voting power received to verify vote extensions; got: %d, expected: >=%d",
			maxPower, requiredVP,
		)
	}
	decoded, err := hex.DecodeString(resBlockHash)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

// ProcessProposal examines the checkpoint in the injected tx of the proposal
// Warning: the returned error of the handler will cause panic of the node,
// therefore we only return error when something really wrong happened
func (h *ProposalHandler) ProcessProposal() sdk.ProcessProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		resAccept := &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}
		resReject := &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}

		k := h.ckptKeeper

		epoch := k.GetEpoch(ctx)
		proposerAddr := sdk.ValAddress(req.ProposerAddress).String()
		// BLS signatures are sent in the last block of the previous epoch,
		// so they should be aggregated in the first block of the new epoch
		// and no BLS signatures are send in epoch 0
		if epoch.IsVoteExtensionProposal(ctx) {
			// 1. extract the special tx containing the checkpoint
			injectedCkpt, err := h.ExtractInjectedCheckpoint(req.Txs)
			if err != nil {
				h.logger.Info(
					"processProposal: failed to extract injected checkpoint from the tx set",
					"height", req.Height, "epoch", epoch.EpochNumber, "proposer", proposerAddr, "err", err)
				// should not return error here as error will cause panic
				return resReject, nil
			}

			// 2. remove the special tx from the request so that
			// the rest of the txs can be handled by the default handler
			req.Txs, err = removeInjectedTx(req.Txs)
			if err != nil {
				// should not return error here as error will cause panic
				h.logger.Info("failed to remove injected tx from request",
					"height", req.Height, "epoch", epoch.EpochNumber, "proposer", proposerAddr, "err", err)
				return resReject, nil
			}

			// 3. verify the validity of the vote extension (2/3 majority is achieved)
			err = baseapp.ValidateVoteExtensions(ctx, h.ckptKeeper, req.Height, ctx.ChainID(), *injectedCkpt.ExtendedCommitInfo)
			if err != nil {
				// the returned err will lead to panic as something very wrong happened during consensus
				h.logger.Info("invalid vote extensions by ValidateVoteExtensions",
					"height", req.Height, "epoch", epoch.EpochNumber, "proposer", proposerAddr, "err", err)
				return resReject, nil
			}

			// 4. rebuild the checkpoint from vote extensions and compare it with
			// the injected checkpoint
			// Note: this is needed because LastBlockID is not available here so that
			// we can't verify whether the injected checkpoint is signing the correct
			// LastBlockID
			// We do not prune invalid vote extensions as this is job of the proposer
			// we just verify that voting power is sufficient to build a checkpoint
			// over 2/3
			ckpt, err := h.buildCheckpointFromVoteExtensions(
				ctx,
				epoch.EpochNumber,
				injectedCkpt.ExtendedCommitInfo,
				h.getValidBlsSigs,
			)
			if err != nil {
				// should not return error here as error will cause panic
				h.logger.Info("invalid vote extensions",
					"height", req.Height, "epoch", epoch.EpochNumber, "proposer", proposerAddr, "err", err)
				return resReject, nil
			}
			// TODO it is possible that although the checkpoints do not match but the injected
			//  checkpoint is still valid. This indicates the existence of a fork (>1/3 malicious voting power)
			//  and we should probably send an alarm and stall the blockchain
			if !ckpt.Equal(injectedCkpt.Ckpt) {
				// should not return error here as error will cause panic
				h.logger.Info("invalid checkpoint in vote extension tx",
					"height", req.Height, "epoch", epoch.EpochNumber, "proposer", proposerAddr, "err", err)
				return resReject, nil
			}
		}

		// 5. verify the rest of the txs using the default handler
		res, err := h.defaultProcessProposalHandler(ctx, req)
		if err != nil {
			h.logger.Error("failed in default ProcessProposal handler",
				"proposer", proposerAddr, "err", err)
			return resReject, fmt.Errorf("failed in default ProcessProposal handler: %w", err)
		}
		if !res.IsAccepted() {
			h.logger.Info("the proposal is rejected by default ProcessProposal handler",
				"height", req.Height, "epoch", epoch.EpochNumber, "proposer", proposerAddr)
			return resReject, nil
		}

		return resAccept, nil
	}
}

// PreBlocker extracts the checkpoint from the injected tx and stores it in the application
// no more validation is needed as it is already done in ProcessProposal
// NOTE: this is appended to the existing PreBlocker in BabylonApp at app.go
func (h *ProposalHandler) PreBlocker() sdk.PreBlocker {
	return func(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
		res := &sdk.ResponsePreBlock{}

		k := h.ckptKeeper
		epoch := k.GetEpoch(ctx)
		// BLS signatures are sent in the last block of the previous epoch,
		// so they should be aggregated in the first block of the new epoch
		// and no BLS signatures are send in epoch 0
		if !epoch.IsVoteExtensionProposal(ctx) {
			return res, nil
		}

		// 1. extract the special tx containing BLS sigs
		injectedCkpt, err := h.ExtractInjectedCheckpoint(req.Txs)
		if err != nil {
			return res, fmt.Errorf(
				"preblocker: failed to extract injected checkpoint from the tx set: %w", err)
		}

		// 2. update checkpoint
		if err := k.SealCheckpoint(ctx, injectedCkpt.Ckpt); err != nil {
			return res, fmt.Errorf("failed to update checkpoint: %w", err)
		}

		return res, nil
	}
}

func (h *ProposalHandler) buildInjectedTxBytes(ckpt *ckpttypes.RawCheckpointWithMeta, info *abci.ExtendedCommitInfo) ([]byte, error) {
	msg := &ckpttypes.MsgInjectedCheckpoint{
		Ckpt:               ckpt,
		ExtendedCommitInfo: info,
	}
	return EncodeMsgsIntoTxBytes(h.txConfig, msg)
}

// ExtractInjectedCheckpoint extracts the injected checkpoint from the tx set
func (h *ProposalHandler) ExtractInjectedCheckpoint(txs [][]byte) (*ckpttypes.MsgInjectedCheckpoint, error) {
	if len(txs) < defaultInjectedTxIndex+1 {
		return nil, fmt.Errorf("the tx set does not contain the injected tx")
	}

	injectedTxBytes := txs[defaultInjectedTxIndex]

	if len(injectedTxBytes) == 0 {
		return nil, fmt.Errorf("the injected vote extensions tx is empty")
	}

	injectedTx, err := h.txConfig.TxDecoder()(injectedTxBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode injected vote extension tx: %w", err)
	}
	msgs := injectedTx.GetMsgs()
	if len(msgs) != 1 {
		return nil, fmt.Errorf("injected tx must have exact one message, got %d", len(msgs))
	}
	injectedCkpt, ok := msgs[0].(*ckpttypes.MsgInjectedCheckpoint)
	if !ok {
		return nil, fmt.Errorf("injected tx must contain MsgInjectedCheckpoint, got %T", msgs[0])
	}

	return injectedCkpt, nil
}

// WithTxVerifier allows to specify the transaction verifier to use in the
// defaultHandler. Useful for testing
func (h *ProposalHandler) WithTxVerifier(ptv baseapp.ProposalTxVerifier) *ProposalHandler {
	defaultHandler := baseapp.NewDefaultProposalHandler(h.mp, ptv)
	h.defaultPrepareProposalHandler = defaultHandler.PrepareProposalHandler()
	h.defaultProcessProposalHandler = defaultHandler.ProcessProposalHandler()
	return h
}

// removeInjectedTx removes the injected tx from the tx set
func removeInjectedTx(txs [][]byte) ([][]byte, error) {
	if len(txs) < defaultInjectedTxIndex+1 {
		return nil, fmt.Errorf("the tx set does not contain the injected tx")
	}

	txs = append(txs[:defaultInjectedTxIndex], txs[defaultInjectedTxIndex+1:]...)

	return txs, nil
}

// EncodeMsgsIntoTxBytes encodes the given msgs into a single transaction.
func EncodeMsgsIntoTxBytes(txConfig client.TxConfig, msgs ...sdk.Msg) ([]byte, error) {
	txBuilder := txConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgs...)
	if err != nil {
		return nil, err
	}

	txBytes, err := txConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, err
	}

	return txBytes, nil
}
