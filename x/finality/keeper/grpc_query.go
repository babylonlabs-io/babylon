package keeper

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/runtime"

	"cosmossdk.io/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

var _ types.QueryServer = Keeper{}

// FinalityProviderPowerAtHeight returns the voting power of the specified finality provider
// at the provided Babylon height
func (k Keeper) FinalityProviderPowerAtHeight(ctx context.Context, req *types.QueryFinalityProviderPowerAtHeightRequest) (*types.QueryFinalityProviderPowerAtHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	fpBTCPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to unmarshal finality provider BTC PK hex: %v", err)
	}

	if !k.BTCStakingKeeper.HasFinalityProvider(ctx, *fpBTCPK) {
		return nil, bstypes.ErrFpNotFound
	}

	store := k.votingPowerBbnBlockHeightStore(ctx, req.Height)
	iter := store.ReverseIterator(nil, nil)
	defer iter.Close()

	if !iter.Valid() {
		return nil, types.ErrVotingPowerTableNotUpdated.Wrapf("height: %d", req.Height)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	power := k.GetVotingPower(sdkCtx, fpBTCPK.MustMarshal(), req.Height)

	return &types.QueryFinalityProviderPowerAtHeightResponse{VotingPower: power}, nil
}

// FinalityProviderCurrentPower returns the voting power of the specified finality provider
// at the current height
func (k Keeper) FinalityProviderCurrentPower(ctx context.Context, req *types.QueryFinalityProviderCurrentPowerRequest) (*types.QueryFinalityProviderCurrentPowerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	fpBTCPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to unmarshal finality provider BTC PK hex: %v", err)
	}

	height, power := k.GetCurrentVotingPower(ctx, *fpBTCPK)

	return &types.QueryFinalityProviderCurrentPowerResponse{Height: height, VotingPower: power}, nil
}

// ActiveFinalityProvidersAtHeight returns the active finality providers at the provided height
func (k Keeper) ActiveFinalityProvidersAtHeight(ctx context.Context, req *types.QueryActiveFinalityProvidersAtHeightRequest) (*types.QueryActiveFinalityProvidersAtHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := k.votingPowerBbnBlockHeightStore(sdkCtx, req.Height)

	var finalityProvidersWithMeta []*bstypes.FinalityProviderWithMeta
	pageRes, err := query.Paginate(store, req.Pagination, func(key, value []byte) error {
		finalityProvider, err := k.BTCStakingKeeper.GetFinalityProvider(sdkCtx, key)
		if err != nil {
			return err
		}

		votingPower := k.GetVotingPower(sdkCtx, key, req.Height)
		if votingPower > 0 {
			finalityProviderWithMeta := bstypes.FinalityProviderWithMeta{
				BtcPk:                finalityProvider.BtcPk,
				Height:               req.Height,
				VotingPower:          votingPower,
				SlashedBabylonHeight: finalityProvider.SlashedBabylonHeight,
				SlashedBtcHeight:     finalityProvider.SlashedBtcHeight,
				HighestVotedHeight:   finalityProvider.HighestVotedHeight,
			}
			finalityProvidersWithMeta = append(finalityProvidersWithMeta, &finalityProviderWithMeta)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.QueryActiveFinalityProvidersAtHeightResponse{FinalityProviders: convertToActiveFinalityProvidersAtHeightResponse(finalityProvidersWithMeta), Pagination: pageRes}, nil
}

// ActivatedHeight returns the Babylon height in which the BTC Staking protocol was enabled
func (k Keeper) ActivatedHeight(ctx context.Context, req *types.QueryActivatedHeightRequest) (*types.QueryActivatedHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	activatedHeight, err := k.GetBTCStakingActivatedHeight(sdkCtx)
	if err != nil {
		return nil, err
	}
	return &types.QueryActivatedHeightResponse{Height: activatedHeight}, nil
}

// ListPublicRandomness returns a list of public randomness committed by a given
// finality provider
func (k Keeper) ListPublicRandomness(ctx context.Context, req *types.QueryListPublicRandomnessRequest) (*types.QueryListPublicRandomnessResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	fpBTCPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to unmarshal finality provider BTC PK hex: %v", err)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := k.pubRandFpStore(sdkCtx, fpBTCPK)
	pubRandMap := map[uint64]*bbn.SchnorrPubRand{}
	pageRes, err := query.Paginate(store, req.Pagination, func(key, value []byte) error {
		height := sdk.BigEndianToUint64(key)
		pubRand, err := bbn.NewSchnorrPubRand(value)
		if err != nil {
			panic("failed to unmarshal EOTS public randomness in KVStore")
		}
		pubRandMap[height] = pubRand
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &types.QueryListPublicRandomnessResponse{
		PubRandMap: pubRandMap,
		Pagination: pageRes,
	}
	return resp, nil
}

// ListPubRandCommit returns a list of public randomness commitment by a given
// finality provider
func (k Keeper) ListPubRandCommit(ctx context.Context, req *types.QueryListPubRandCommitRequest) (*types.QueryListPubRandCommitResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	fpBTCPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to unmarshal finality provider BTC PK hex: %v", err)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := k.pubRandCommitFpStore(sdkCtx, fpBTCPK)
	pubRandCommitMap := map[uint64]*types.PubRandCommitResponse{}
	pageRes, err := query.Paginate(store, req.Pagination, func(key, value []byte) error {
		height := sdk.BigEndianToUint64(key)
		var prCommit types.PubRandCommit
		k.cdc.MustUnmarshal(value, &prCommit)
		pubRandCommitMap[height] = prCommit.ToResponse()
		return nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &types.QueryListPubRandCommitResponse{
		PubRandCommitMap: pubRandCommitMap,
		Pagination:       pageRes,
	}
	return resp, nil
}

func (k Keeper) Block(ctx context.Context, req *types.QueryBlockRequest) (*types.QueryBlockResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	b, err := k.GetBlock(sdkCtx, req.Height)
	if err != nil {
		return nil, err
	}

	return &types.QueryBlockResponse{Block: b}, nil
}

// ListBlocks returns a list of blocks at the given finalisation status
func (k Keeper) ListBlocks(ctx context.Context, req *types.QueryListBlocksRequest) (*types.QueryListBlocksResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := k.blockStore(sdkCtx)
	var ibs []*types.IndexedBlock
	pageRes, err := query.FilteredPaginate(store, req.Pagination, func(_ []byte, value []byte, accumulate bool) (bool, error) {
		var ib types.IndexedBlock
		k.cdc.MustUnmarshal(value, &ib)

		// hit if the queried status matches the block status, or the querier wants blocks in any state
		if (req.Status == types.QueriedBlockStatus_FINALIZED && ib.Finalized) ||
			(req.Status == types.QueriedBlockStatus_NON_FINALIZED && !ib.Finalized) ||
			(req.Status == types.QueriedBlockStatus_ANY) {
			if accumulate {
				ibs = append(ibs, &ib)
			}
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &types.QueryListBlocksResponse{
		Blocks:     ibs,
		Pagination: pageRes,
	}
	return resp, nil
}

// VotesAtHeight returns the set of votes at a given Babylon height
func (k Keeper) VotesAtHeight(ctx context.Context, req *types.QueryVotesAtHeightRequest) (*types.QueryVotesAtHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// get the sig set of babylon block at given height
	btcPks := []bbn.BIP340PubKey{}
	sigSet := k.GetSigSet(sdkCtx, req.Height)
	for pkHex := range sigSet {
		pk, err := bbn.NewBIP340PubKeyFromHex(pkHex)
		if err != nil {
			// failing to unmarshal finality provider BTC PK in KVStore is a programming error
			panic(fmt.Errorf("%w: %w", bbn.ErrUnmarshal, err))
		}

		btcPks = append(btcPks, pk.MustMarshal())
	}

	return &types.QueryVotesAtHeightResponse{BtcPks: btcPks}, nil
}

// Evidence returns the first evidence that allows to extract the finality provider's SK
// associated with the given finality provider's PK.
func (k Keeper) Evidence(ctx context.Context, req *types.QueryEvidenceRequest) (*types.QueryEvidenceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	fpBTCPK, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to unmarshal finality provider BTC PK hex: %v", err)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	evidence := k.GetFirstSlashableEvidence(sdkCtx, fpBTCPK)
	if evidence == nil {
		return nil, types.ErrNoSlashableEvidence
	}

	resp := &types.QueryEvidenceResponse{
		Evidence: convertToEvidenceResponse(evidence),
	}
	return resp, nil
}

// ListEvidences returns a list of evidences
func (k Keeper) ListEvidences(ctx context.Context, req *types.QueryListEvidencesRequest) (*types.QueryListEvidencesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var evidences []*types.Evidence

	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	eStore := prefix.NewStore(storeAdapter, types.EvidenceKey)

	pageRes, err := query.FilteredPaginate(eStore, req.Pagination, func(key []byte, _ []byte, accumulate bool) (bool, error) {
		// NOTE: we have to strip the rest bytes after the first 32 bytes
		// since there is another layer of KVStore (height -> evidence) under eStore
		// in which height is uint64 thus takes 8 bytes
		strippedKey := key[:bbn.BIP340PubKeyLen]
		fpBTCPK, err := bbn.NewBIP340PubKey(strippedKey)
		if err != nil {
			panic(err) // failing to unmarshal fpBTCPK in KVStore can only be a programming error
		}
		evidence := k.GetFirstSlashableEvidence(sdkCtx, fpBTCPK)

		// hit if the finality provider has a full evidence of equivocation
		if evidence != nil && evidence.BlockHeight >= req.StartHeight {
			if accumulate {
				evidences = append(evidences, evidence)
			}
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &types.QueryListEvidencesResponse{
		Evidences:  convertToEvidenceListResponse(evidences),
		Pagination: pageRes,
	}
	return resp, nil
}

// SigningInfo returns signing-info of a specific finality provider.
func (k Keeper) SigningInfo(ctx context.Context, req *types.QuerySigningInfoRequest) (*types.QuerySigningInfoResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	if req.FpBtcPkHex == "" {
		return nil, status.Errorf(codes.InvalidArgument, "empty finality provider public key")
	}

	fpPk, err := bbn.NewBIP340PubKeyFromHex(req.FpBtcPkHex)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid finality provider public key")
	}

	signingInfo, err := k.FinalityProviderSigningTracker.Get(ctx, fpPk.MustMarshal())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "SigningInfo not found for the finality provider %s", req.FpBtcPkHex)
	}

	return &types.QuerySigningInfoResponse{SigningInfo: convertToSigningInfoResponse(signingInfo)}, nil
}

// SigningInfos returns signing-infos of all finality providers.
func (k Keeper) SigningInfos(ctx context.Context, req *types.QuerySigningInfosRequest) (*types.QuerySigningInfosResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request")
	}

	store := k.storeService.OpenKVStore(ctx)
	var signInfos []types.FinalityProviderSigningInfo

	signingInfoStore := prefix.NewStore(runtime.KVStoreAdapter(store), types.FinalityProviderSigningInfoKeyPrefix)
	pageRes, err := query.Paginate(signingInfoStore, req.Pagination, func(key, value []byte) error {
		var info types.FinalityProviderSigningInfo
		err := k.cdc.Unmarshal(value, &info)
		if err != nil {
			return err
		}
		signInfos = append(signInfos, info)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &types.QuerySigningInfosResponse{SigningInfos: convertToSigningInfosResponse(signInfos), Pagination: pageRes}, nil
}

func convertToSigningInfoResponse(info types.FinalityProviderSigningInfo) types.SigningInfoResponse {
	return types.SigningInfoResponse{
		FpBtcPkHex:          info.FpBtcPk.MarshalHex(),
		StartHeight:         info.StartHeight,
		MissedBlocksCounter: info.MissedBlocksCounter,
		JailedUntil:         info.JailedUntil,
	}
}

func convertToSigningInfosResponse(signInfos []types.FinalityProviderSigningInfo) []types.SigningInfoResponse {
	response := make([]types.SigningInfoResponse, len(signInfos))
	for i, info := range signInfos {
		response[i] = convertToSigningInfoResponse(info)
	}
	return response
}

func convertToEvidenceResponse(evidence *types.Evidence) *types.EvidenceResponse {
	return &types.EvidenceResponse{
		FpBtcPkHex:           evidence.FpBtcPk.MarshalHex(),
		BlockHeight:          evidence.BlockHeight,
		PubRand:              evidence.PubRand,
		CanonicalAppHash:     evidence.CanonicalAppHash,
		ForkAppHash:          evidence.ForkAppHash,
		CanonicalFinalitySig: evidence.CanonicalFinalitySig,
		ForkFinalitySig:      evidence.ForkFinalitySig,
	}
}

func convertToEvidenceListResponse(evidences []*types.Evidence) []*types.EvidenceResponse {
	response := make([]*types.EvidenceResponse, len(evidences))
	for i, evidence := range evidences {
		resp := convertToEvidenceResponse(evidence)
		response[i] = resp
	}
	return response
}

func convertToActiveFinalityProvidersAtHeightResponse(finalityProvidersWithMeta []*bstypes.FinalityProviderWithMeta) []*types.ActiveFinalityProvidersAtHeightResponse {
	var activeFinalityProvidersAtHeightResponse []*types.ActiveFinalityProvidersAtHeightResponse
	for _, fpWithMeta := range finalityProvidersWithMeta {
		activeFinalityProvidersAtHeightResponse = append(activeFinalityProvidersAtHeightResponse, &types.ActiveFinalityProvidersAtHeightResponse{
			BtcPkHex:             fpWithMeta.BtcPk,
			Height:               fpWithMeta.Height,
			VotingPower:          fpWithMeta.VotingPower,
			SlashedBabylonHeight: fpWithMeta.SlashedBabylonHeight,
			SlashedBtcHeight:     fpWithMeta.SlashedBtcHeight,
			Jailed:               fpWithMeta.Jailed,
			HighestVotedHeight:   fpWithMeta.HighestVotedHeight,
		})
	}
	return activeFinalityProvidersAtHeightResponse
}
