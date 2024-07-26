package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	btcstk "github.com/babylonlabs-io/babylon/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

// InitGenesis initializes the keeper state from a provided initial genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	for _, idxBlock := range gs.IndexedBlocks {
		k.SetBlock(ctx, idxBlock)
	}

	for _, evidence := range gs.Evidences {
		k.SetEvidence(ctx, evidence)
	}

	for _, voteSig := range gs.VoteSigs {
		k.SetSig(ctx, voteSig.BlockHeight, voteSig.FpBtcPk, voteSig.FinalitySig)
	}

	for _, pubRand := range gs.PublicRandomness {
		k.SetPubRand(ctx, pubRand.FpBtcPk, pubRand.BlockHeight, *pubRand.PubRand)
	}

	for _, prc := range gs.PubRandCommit {
		k.SetPubRandCommit(ctx, prc.FpBtcPk, prc.PubRandCommit)
	}

	for _, info := range gs.SigningInfos {
		err := k.FinalityProviderSigningTracker.Set(ctx, info.FpBtcPk.MustMarshal(), info.FpSigningInfo)
		if err != nil {
			return err
		}
	}

	for _, array := range gs.MissedBlocks {
		for _, missed := range array.MissedBlocks {
			if err := k.SetMissedBlockBitmapValue(ctx, array.FpBtcPk, missed.Index, missed.Missed); err != nil {
				return err
			}
		}
	}

	return k.SetParams(ctx, gs.Params)
}

// ExportGenesis returns the keeper state into a exported genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	blocks, err := k.blocks(ctx)
	if err != nil {
		return nil, err
	}

	evidences, err := k.evidences(ctx)
	if err != nil {
		return nil, err
	}

	voteSigs, err := k.voteSigs(ctx)
	if err != nil {
		return nil, err
	}

	pubRandomness, err := k.publicRandomness(ctx)
	if err != nil {
		return nil, err
	}

	prCommit, err := k.exportPubRandCommit(ctx)
	if err != nil {
		return nil, err
	}

	signingInfos, missedBlocks, err := k.signingInfosAndMissedBlock(ctx)
	if err != nil {
		return nil, err
	}

	return &types.GenesisState{
		Params:           k.GetParams(ctx),
		IndexedBlocks:    blocks,
		Evidences:        evidences,
		VoteSigs:         voteSigs,
		PublicRandomness: pubRandomness,
		PubRandCommit:    prCommit,
		SigningInfos:     signingInfos,
		MissedBlocks:     missedBlocks,
	}, nil
}

// blocks loads all blocks stored.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) blocks(ctx context.Context) ([]*types.IndexedBlock, error) {
	blocks := make([]*types.IndexedBlock, 0)

	iter := k.blockStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var blk types.IndexedBlock
		if err := k.cdc.Unmarshal(iter.Value(), &blk); err != nil {
			return nil, err
		}
		blocks = append(blocks, &blk)
	}

	return blocks, nil
}

// evidences loads all evidences stored.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) evidences(ctx context.Context) (evidences []*types.Evidence, err error) {
	evidences = make([]*types.Evidence, 0)

	iter := k.evidenceStore(ctx).Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var evd types.Evidence
		if err := k.cdc.Unmarshal(iter.Value(), &evd); err != nil {
			return nil, err
		}
		evidences = append(evidences, &evd)
	}

	return evidences, nil
}

// voteSigs iterates over all votes on the store, parses the height and the finality provider
// public key from the iterator key and the finality signature from the iterator value.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) voteSigs(ctx context.Context) ([]*types.VoteSig, error) {
	store := k.voteStore(ctx)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	voteSigs := make([]*types.VoteSig, 0)
	for ; iter.Valid(); iter.Next() {
		// key contains the height and the fp
		blkHeight, fpBTCPK, err := btcstk.ParseBlkHeightAndPubKeyFromStoreKey(iter.Key())
		if err != nil {
			return nil, err
		}
		finalitySig, err := bbn.NewSchnorrEOTSSig(iter.Value())
		if err != nil {
			return nil, err
		}

		voteSigs = append(voteSigs, &types.VoteSig{
			BlockHeight: blkHeight,
			FpBtcPk:     fpBTCPK,
			FinalitySig: finalitySig,
		})
	}

	return voteSigs, nil
}

// publicRandomness iterates over all commited randoms on the store, parses the finality provider public key
// and the height from the iterator key and the commited random from the iterator value.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) publicRandomness(ctx context.Context) ([]*types.PublicRandomness, error) {
	store := k.pubRandStore(ctx)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	commtRandoms := make([]*types.PublicRandomness, 0)
	for ; iter.Valid(); iter.Next() {
		// key contains the fp and the block height
		fpBTCPK, blkHeight, err := parsePubKeyAndBlkHeightFromStoreKey(iter.Key())
		if err != nil {
			return nil, err
		}
		pubRand, err := bbn.NewSchnorrPubRand(iter.Value())
		if err != nil {
			return nil, err
		}

		commtRandoms = append(commtRandoms, &types.PublicRandomness{
			BlockHeight: blkHeight,
			FpBtcPk:     fpBTCPK,
			PubRand:     pubRand,
		})
	}

	return commtRandoms, nil
}

// exportPubRandCommit iterates over all public randomness commitment on the store,
// parses the finality provider public key and the height from the iterator key
// and the commitment from the iterator value.
// This function has high resource consumption and should be only used on export genesis.
func (k Keeper) exportPubRandCommit(ctx context.Context) ([]*types.PubRandCommitWithPK, error) {
	store := k.pubRandCommitStore(ctx)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	commtRandoms := make([]*types.PubRandCommitWithPK, 0)
	for ; iter.Valid(); iter.Next() {
		// key contains the fp and the block height
		fpBTCPK, _, err := parsePubKeyAndBlkHeightFromStoreKey(iter.Key())
		if err != nil {
			return nil, err
		}
		var prc types.PubRandCommit
		k.cdc.MustUnmarshal(iter.Value(), &prc)

		commtRandoms = append(commtRandoms, &types.PubRandCommitWithPK{
			FpBtcPk:       fpBTCPK,
			PubRandCommit: &prc,
		})
	}

	return commtRandoms, nil
}

func (k Keeper) signingInfosAndMissedBlock(ctx context.Context) ([]types.SigningInfo, []types.FinalityProviderMissedBlocks, error) {
	signingInfos := make([]types.SigningInfo, 0)
	missedBlocks := make([]types.FinalityProviderMissedBlocks, 0)
	err := k.FinalityProviderSigningTracker.Walk(ctx, nil, func(fpPkBytes []byte, info types.FinalityProviderSigningInfo) (stop bool, err error) {
		fpPk, err := bbn.NewBIP340PubKey(fpPkBytes)
		if err != nil {
			return true, err
		}

		signingInfos = append(signingInfos, types.SigningInfo{
			FpBtcPk:       fpPk,
			FpSigningInfo: info,
		})

		localMissedBlocks, err := k.GetFinalityProviderMissedBlocks(ctx, fpPk)
		if err != nil {
			return true, err
		}

		missedBlocks = append(missedBlocks, types.FinalityProviderMissedBlocks{
			FpBtcPk:      fpPk,
			MissedBlocks: localMissedBlocks,
		})

		return false, nil
	})
	if err != nil {
		return nil, nil, err
	}

	return signingInfos, missedBlocks, nil
}

// parsePubKeyAndBlkHeightFromStoreKey expects to receive a key with
// BIP340PubKey(fpBTCPK) || BigEndianUint64(blkHeight)
func parsePubKeyAndBlkHeightFromStoreKey(key []byte) (fpBTCPK *bbn.BIP340PubKey, blkHeight uint64, err error) {
	sizeBigEndian := 8
	keyLen := len(key)
	if keyLen < sizeBigEndian+1 {
		return nil, 0, fmt.Errorf("key not long enough to parse BIP340PubKey and block height: %s", key)
	}

	startKeyHeight := keyLen - sizeBigEndian
	fpBTCPK, err = bbn.NewBIP340PubKey(key[:startKeyHeight])
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse pub key from key %w: %w", bbn.ErrUnmarshal, err)
	}

	blkHeight = sdk.BigEndianToUint64(key[startKeyHeight:])
	return fpBTCPK, blkHeight, nil
}
