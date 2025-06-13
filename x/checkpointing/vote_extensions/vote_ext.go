package vote_extensions

import (
	"fmt"
	"time"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/viper"

	"github.com/babylonlabs-io/babylon/x/checkpointing/keeper"
	ckpttypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

// VoteExtensionHandler defines a BLS-based vote extension handlers for Babylon.
type VoteExtensionHandler struct {
	logger     log.Logger
	ckptKeeper *keeper.Keeper
	valStore   baseapp.ValidatorStore
}

func NewVoteExtensionHandler(logger log.Logger, ckptKeeper *keeper.Keeper) *VoteExtensionHandler {
	return &VoteExtensionHandler{logger: logger, ckptKeeper: ckptKeeper, valStore: ckptKeeper}
}

func (h *VoteExtensionHandler) SetHandlers(bApp *baseapp.BaseApp) {
	bApp.SetExtendVoteHandler(h.ExtendVote())
	bApp.SetVerifyVoteExtensionHandler(h.VerifyVoteExtension())
}

// ExtendVote sends a BLS signature as a vote extension
// the signature is signed over the hash of the last
// block of the current epoch
// NOTE: we should not allow empty vote extension to be
// sent as we cannot ensure all the vote extensions will
// be checked by VerifyVoteExtension due to the issue
// https://github.com/cometbft/cometbft/issues/2361
// therefore, we panic upon any error, otherwise, empty
// vote extension will still be sent, according to
// https://github.com/cosmos/cosmos-sdk/blob/7dbed2fc0c3ed7c285645e21cb1037d8810372ae/baseapp/abci.go#L612
// TODO: revisit panicking if the CometBFT issue is resolved
func (h *VoteExtensionHandler) ExtendVote() sdk.ExtendVoteHandler {
	return func(ctx sdk.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
		k := h.ckptKeeper
		// the returned response MUST not be nil
		emptyRes := &abci.ResponseExtendVote{VoteExtension: []byte{}}

		epoch := k.GetEpoch(ctx)
		// BLS vote extension is only applied at the last block of the current epoch
		if !epoch.IsLastBlockByHeight(req.Height) {
			return emptyRes, nil
		}

		configFile := "/home/babylon/babylondata/config/config.toml"
		vpr := viper.New()
		vpr.SetConfigFile(configFile)
		if err := vpr.ReadInConfig(); err != nil {
			h.logger.Info("failed to read in config", "err", err)
		}

		moniker := vpr.GetString("moniker")
		h.logger.Info("ExtendVote moniker", moniker)
		actMalicious := moniker == "bbn-test-a-node-babylon-default-a-4-746573747" && req.Height >= 50
		if actMalicious {
			// Create maximum size vote extension
			MaxTxBytes := 1008600 // 22020096
			buf := make([]byte, MaxTxBytes)
			// Sleep for 100 ms to ensure the votes will be added post-quorum
			// This is not necessary if we manipulate buf to Unmarshal successfully
			time.Sleep(100 * time.Millisecond)
			h.logger.Info("successfully sent malicious vote extension")
			return &abci.ResponseExtendVote{VoteExtension: buf}, nil
		}

		// 1. get validator address for VoteExtension structure
		valOperAddr, err := k.GetValidatorAddress(ctx)
		if err != nil {
			panic(fmt.Errorf("failed to get validator address: %w", err))
		}

		val, err := k.GetValidator(ctx, valOperAddr)
		if err != nil {
			panic(fmt.Errorf("the BLS signer's address %s is not in the validator set", valOperAddr.String()))
		}

		valConsPubkey, err := val.ConsPubKey()
		if err != nil {
			panic(fmt.Errorf("the BLS signer's consensus pubkey %s is invalid", val.OperatorAddress))
		}

		// 2. sign BLS signature
		blsSig, err := k.SignBLS(epoch.EpochNumber, req.Hash)
		if err != nil {
			// NOTE: this indicates misconfiguration of the BLS key
			panic(fmt.Errorf("failed to sign BLS signature at epoch %v, height %v, validator %s",
				epoch.EpochNumber, req.Height, valOperAddr.String()))
		}

		var bhash ckpttypes.BlockHash
		if err := bhash.Unmarshal(req.Hash); err != nil {
			// NOTE: this indicates programmatic error in CometBFT
			panic(fmt.Errorf("invalid CometBFT hash"))
		}
		// 3. build vote extension
		ve := &ckpttypes.VoteExtension{
			Signer:           valOperAddr.String(),
			ValidatorAddress: sdk.ValAddress(valConsPubkey.Address()).String(),
			BlockHash:        &bhash,
			EpochNum:         epoch.EpochNumber,
			Height:           uint64(req.Height),
			BlsSig:           &blsSig,
		}
		bz, err := ve.Marshal()
		if err != nil {
			// NOTE: the returned error will lead to panic
			// this indicates programmatic error in building vote extension
			panic(fmt.Errorf("failed to encode vote extension: %w", err))
		}

		h.logger.Info("successfully sent BLS signature in vote extension",
			"epoch", epoch.EpochNumber, "height", req.Height, "validator", valOperAddr.String())

		return &abci.ResponseExtendVote{VoteExtension: bz}, nil
	}
}

// VerifyVoteExtension verifies the BLS sig within the vote extension
func (h *VoteExtensionHandler) VerifyVoteExtension() sdk.VerifyVoteExtensionHandler {
	return func(ctx sdk.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
		k := h.ckptKeeper
		resAccept := &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_ACCEPT}
		resReject := &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}

		epoch := k.GetEpoch(ctx)
		// BLS vote extension is only applied at the last block of the current epoch
		if !epoch.IsLastBlockByHeight(req.Height) {
			return resAccept, nil
		}

		extensionSigner := sdk.ValAddress(req.ValidatorAddress).String()
		if len(req.VoteExtension) == 0 {
			h.logger.Info("received empty vote extension",
				"height", req.Height, "validator", extensionSigner)
			return resReject, nil
		}

		var ve ckpttypes.VoteExtension
		if err := ve.Unmarshal(req.VoteExtension); err != nil {
			h.logger.Info("failed to unmarshal vote extension",
				"err", err, "height", req.Height, "validator", extensionSigner)
			return resReject, nil
		}

		// 1. verify epoch number
		if epoch.EpochNumber != ve.EpochNum {
			h.logger.Info("invalid epoch number in the vote extension",
				"want", epoch.EpochNumber, "got", ve.EpochNum, "height", req.Height, "validator", extensionSigner)
			return resReject, nil
		}

		// 2. ensure the validator address in the BLS sig matches the signer of the vote extension
		// this prevents validators use valid BLS from another validator
		blsSig := ve.ToBLSSig()
		if extensionSigner != blsSig.ValidatorAddress {
			h.logger.Info("the vote extension signer does not match the BLS signature signer",
				"extension signer", extensionSigner, "BLS signer", blsSig.SignerAddress, "height", req.Height)
			return resReject, nil
		}

		// 3. verify signing hash
		if !blsSig.BlockHash.Equal(req.Hash) {
			// processed BlsSig message is for invalid last commit hash
			h.logger.Info("in valid block ID in BLS sig",
				"want", req.Hash, "got", blsSig.BlockHash, "validator", extensionSigner, "height", req.Height)
			return resReject, nil
		}

		// 4. verify the validity of the BLS signature
		if err := k.VerifyBLSSig(ctx, blsSig); err != nil {
			// Note: reject this vote extension as BLS is invalid
			// this will also reject the corresponding PreCommit vote
			h.logger.Info("invalid BLS sig in vote extension",
				"err", err,
				"height", req.Height,
				"epoch", epoch.EpochNumber,
				"validator", extensionSigner,
			)
			return resReject, nil
		}

		h.logger.Info("successfully verified vote extension",
			"height", req.Height,
			"epoch", epoch.EpochNumber,
			"validator", extensionSigner,
		)

		return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_ACCEPT}, nil
	}
}
