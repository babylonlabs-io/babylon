package keeper

import (
	"context"
	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	k.SetPort(ctx, gs.PortId)

	for _, ci := range gs.ChainsInfo {
		k.setChainInfo(ctx, ci)
	}

	for _, h := range gs.ChainsIndexedHeaders {
		if err := k.insertHeader(ctx, h.ConsumerId, h); err != nil {
			return err
		}
	}

	for _, f := range gs.ChainsForks {
		for _, h := range f.Headers {
			if err := k.insertForkHeader(ctx, h.ConsumerId, h); err != nil {
				return err
			}
		}
	}

	for _, ei := range gs.ChainsEpochsInfo {
		k.setEpochChainInfo(ctx, ei.ChainInfo.ChainInfo.ConsumerId, ei.EpochNumber, ei.ChainInfo)
	}

	if gs.LastSentSegment != nil {
		k.setLastSentSegment(ctx, gs.LastSentSegment)
	}

	for _, se := range gs.SealedEpochsProofs {
		k.sealedEpochProofStore(ctx).Set(
			sdk.Uint64ToBigEndian(se.EpochNumber),
			k.cdc.MustMarshal(se.Proof),
		)
	}

	return k.SetParams(ctx, gs.Params)
}

// ExportGenesis returns the keeper state into a exported genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	consumerIDs, ci := k.chainsInfo(ctx)

	h, err := k.chainsHeaders(ctx, consumerIDs)
	if err != nil {
		return nil, err
	}

	f, err := k.chainsForks(ctx, consumerIDs)
	if err != nil {
		return nil, err
	}

	ei, err := k.chainsEpochsInfo(ctx, consumerIDs)
	if err != nil {
		return nil, err
	}

	se, err := k.sealedEpochsProofs(ctx)
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		Params:               k.GetParams(ctx),
		PortId:               k.GetPort(ctx),
		ChainsInfo:           ci,
		ChainsIndexedHeaders: h,
		ChainsForks:          f,
		ChainsEpochsInfo:     ei,
		LastSentSegment:      k.GetLastSentSegment(ctx),
		SealedEpochsProofs:   se,
	}, nil
}

// chainsInfo gets the information and consumer ID of all consumer chains
// that integrate Babylon
func (k Keeper) chainsInfo(ctx context.Context) ([]string, []*types.ChainInfo) {
	consumerIds := make([]string, 0)
	chains := make([]*types.ChainInfo, 0)
	iter := k.chainInfoStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		// consumer ID is the store key
		consumerIds = append(consumerIds, string(iter.Key()))

		var chainInfo types.ChainInfo
		k.cdc.MustUnmarshal(iter.Value(), &chainInfo)
		chains = append(chains, &chainInfo)
	}
	return consumerIds, chains
}

func (k Keeper) chainsHeaders(ctx context.Context, consumerIDs []string) ([]*types.IndexedHeader, error) {
	headers := make([]*types.IndexedHeader, 0)
	for _, cID := range consumerIDs {
		hs, err := k.headersByChain(ctx, cID)
		if err != nil {
			return nil, err
		}
		headers = append(headers, hs...)
	}
	return headers, nil
}

func (k Keeper) headersByChain(ctx context.Context, consumerID string) ([]*types.IndexedHeader, error) {
	headers := make([]*types.IndexedHeader, 0)
	iter := k.canonicalChainStore(ctx, consumerID).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var h types.IndexedHeader
		if err := k.cdc.Unmarshal(iter.Value(), &h); err != nil {
			return nil, err
		}

		if err := h.Validate(); err != nil {
			return nil, err
		}
		headers = append(headers, &h)
	}
	return headers, nil
}

func (k Keeper) chainsEpochsInfo(ctx context.Context, consumerIDs []string) ([]*types.EpochChainInfoEntry, error) {
	entries := make([]*types.EpochChainInfoEntry, 0)
	for _, cID := range consumerIDs {
		epochsInfo, err := k.epochsInfoByChain(ctx, cID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, epochsInfo...)
	}
	return entries, nil
}

func (k Keeper) epochsInfoByChain(ctx context.Context, consumerID string) ([]*types.EpochChainInfoEntry, error) {
	entries := make([]*types.EpochChainInfoEntry, 0)
	iter := k.epochChainInfoStore(ctx, consumerID).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		epochNum := sdk.BigEndianToUint64(iter.Key())

		var ci types.ChainInfoWithProof
		if err := k.cdc.Unmarshal(iter.Value(), &ci); err != nil {
			return nil, err
		}
		entry := &types.EpochChainInfoEntry{
			EpochNumber: epochNum,
			ChainInfo:   &ci,
		}
		if err := entry.Validate(); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (k Keeper) chainsForks(ctx context.Context, consumerIDs []string) ([]*types.Forks, error) {
	forks := make([]*types.Forks, 0)
	for _, cID := range consumerIDs {
		fs, err := k.forksByChain(ctx, cID)
		if err != nil {
			return nil, err
		}
		forks = append(forks, fs...)
	}
	return forks, nil
}

func (k Keeper) forksByChain(ctx context.Context, consumerID string) ([]*types.Forks, error) {
	forks := make([]*types.Forks, 0)
	iter := k.forkStore(ctx, consumerID).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var f types.Forks
		if err := k.cdc.Unmarshal(iter.Value(), &f); err != nil {
			return nil, err
		}

		if err := f.Validate(); err != nil {
			return nil, err
		}
		forks = append(forks, &f)
	}
	return forks, nil
}

func (k Keeper) sealedEpochsProofs(ctx context.Context) ([]*types.SealedEpochProofEntry, error) {
	entries := make([]*types.SealedEpochProofEntry, 0)
	iter := k.sealedEpochProofStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		epochNum := sdk.BigEndianToUint64(iter.Key())

		var proof types.ProofEpochSealed
		if err := k.cdc.Unmarshal(iter.Value(), &proof); err != nil {
			return nil, err
		}
		entry := &types.SealedEpochProofEntry{
			EpochNumber: epochNum,
			Proof:       &proof,
		}
		if err := entry.Validate(); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
