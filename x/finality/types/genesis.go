package types

import (
	fmt "fmt"
	"sort"

	"github.com/babylonlabs-io/babylon/v4/types"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := types.ValidateEntries(gs.IndexedBlocks, func(b *IndexedBlock) uint64 {
		return b.Height
	}); err != nil {
		return err
	}

	for _, e := range gs.Evidences {
		if err := e.ValidateBasic(); err != nil {
			return err
		}
	}

	if err := types.ValidateEntries(gs.VoteSigs, func(v *VoteSig) string {
		bz := make([]byte, 0)
		// avoid panic here. If FpBtcPk is nil,
		// will throw corresponding error later
		if v.FpBtcPk != nil {
			bz, _ = v.FpBtcPk.Marshal() // returned err is always nil
		}
		// unique key is FPs BTC PK and block height
		bz = append(bz, byte(v.BlockHeight))
		return string(bz)
	}); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.PublicRandomness, func(pr *PublicRandomness) string {
		bz := make([]byte, 0)
		// avoid panic here. If FpBtcPk is nil,
		// will throw corresponding error later
		if pr.FpBtcPk != nil {
			bz, _ = pr.FpBtcPk.Marshal() // returned err is always nil
		}
		// unique key is FPs BTC PK and block height
		bz = append(bz, byte(pr.BlockHeight))
		return string(bz)
	}); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.PubRandCommit, func(pr *PubRandCommitWithPK) string {
		// unique key is FPs BTC PK and start height
		var (
			bz          = make([]byte, 0)
			startHeight uint64
		)
		// avoid panic here
		// will throw corresponding error later
		if pr.FpBtcPk != nil {
			bz, _ = pr.FpBtcPk.Marshal() // returned err is always nil
		}
		if pr.PubRandCommit != nil {
			startHeight = pr.PubRandCommit.StartHeight
		}
		bz = append(bz, byte(startHeight))
		return string(bz)
	}); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.SigningInfos, func(si SigningInfo) string {
		var pk string
		// avoid panic here. If FpBtcPk is nil,
		// will throw corresponding error later
		if si.FpBtcPk != nil {
			pk = si.FpBtcPk.MarshalHex()
		}
		return pk
	}); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.MissedBlocks, func(mb FinalityProviderMissedBlocks) string {
		var pk string
		// avoid panic here. If FpBtcPk is nil,
		// will throw corresponding error later
		if mb.FpBtcPk != nil {
			pk = mb.FpBtcPk.MarshalHex()
		}
		return pk
	}); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.VotingPowers, func(vp *VotingPowerFP) string {
		bz := make([]byte, 0)
		// avoid panic here. If FpBtcPk is nil,
		// will throw corresponding error later
		if vp.FpBtcPk != nil {
			bz, _ = vp.FpBtcPk.Marshal() // returned err is always nil
		}
		// unique key is FPs BTC PK and block height
		bz = append(bz, byte(vp.BlockHeight))
		return string(bz)
	}); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.VpDstCache, func(vpdc *VotingPowerDistCacheBlkHeight) uint64 {
		return vpdc.BlockHeight
	}); err != nil {
		return err
	}

	if gs.NextHeightToReward > gs.NextHeightToFinalize {
		return fmt.Errorf("invalid genesis state. Next height to reward %d is higher than next height to finalize %d", gs.NextHeightToReward, gs.NextHeightToFinalize)
	}

	return gs.Params.Validate()
}

func (v VoteSig) Validate() error {
	if v.FpBtcPk == nil {
		return ErrInvalidFinalitySig.Wrap("empty finality provider BTC public key")
	}

	if v.FinalitySig == nil {
		return ErrInvalidFinalitySig.Wrap("empty finality signature")
	}

	if v.FpBtcPk.Size() != types.BIP340PubKeyLen {
		return ErrInvalidFinalitySig.Wrapf("invalid finality provider BTC public key length: got %d, want %d", v.FpBtcPk.Size(), types.BIP340PubKeyLen)
	}

	if v.FinalitySig.Size() != types.SchnorrEOTSSigLen {
		return ErrInvalidFinalitySig.Wrapf("invalid finality signature length: got %d, want %d", v.FinalitySig.Size(), types.SchnorrEOTSSigLen)
	}
	return nil
}

func (pr PublicRandomness) Validate() error {
	if pr.FpBtcPk == nil {
		return ErrInvalidPubRand.Wrap("empty finality provider BTC public key")
	}
	if pr.PubRand == nil {
		return ErrInvalidPubRand.Wrap("empty public randomness")
	}
	if pr.FpBtcPk.Size() != types.BIP340PubKeyLen {
		return ErrInvalidPubRand.Wrapf("finality provider BTC public key length: got %d, want %d", pr.FpBtcPk.Size(), types.BIP340PubKeyLen)
	}

	if pr.PubRand.Size() != types.SchnorrPubRandLen {
		return ErrInvalidPubRand.Wrapf("invalid public randomnes length: got %d, want %d", pr.PubRand.Size(), types.SchnorrPubRandLen)
	}
	return nil
}

func (prc PubRandCommitWithPK) Validate() error {
	if prc.FpBtcPk == nil {
		return ErrInvalidPubRandCommit.Wrap("empty finality provider BTC public key")
	}
	if prc.PubRandCommit == nil {
		return ErrInvalidPubRandCommit.Wrap("empty commitment")
	}
	if prc.FpBtcPk.Size() != types.BIP340PubKeyLen {
		return ErrInvalidPubRandCommit.Wrapf("finality provider BTC public key length: got %d, want %d", prc.FpBtcPk.Size(), types.BIP340PubKeyLen)
	}

	return prc.PubRandCommit.Validate()
}

func (si SigningInfo) Validate() error {
	if si.FpBtcPk == nil {
		return fmt.Errorf("invalid signing info. empty finality provider BTC public key")
	}

	if si.FpBtcPk.Size() != types.BIP340PubKeyLen {
		return fmt.Errorf("invalid signing info. finality provider BTC public key length: got %d, want %d", si.FpBtcPk.Size(), types.BIP340PubKeyLen)
	}

	if !si.FpBtcPk.Equals(si.FpSigningInfo.FpBtcPk) {
		return fmt.Errorf("invalid signing info. finality provider BTC does not match: got %s, want %s", si.FpBtcPk.MarshalHex(), si.FpSigningInfo.FpBtcPk.MarshalHex())
	}

	return si.FpSigningInfo.Validate()
}

func (fpmb FinalityProviderMissedBlocks) Validate() error {
	if fpmb.FpBtcPk == nil {
		return fmt.Errorf("invalid fp missed blocks. empty finality provider BTC public key")
	}

	if len(fpmb.MissedBlocks) == 0 {
		return fmt.Errorf("invalid fp missed blocks. empty missed blocks")
	}

	if fpmb.FpBtcPk.Size() != types.BIP340PubKeyLen {
		return fmt.Errorf("invalid fp missed blocks. finality provider BTC public key length: got %d, want %d", fpmb.FpBtcPk.Size(), types.BIP340PubKeyLen)
	}

	for _, mb := range fpmb.MissedBlocks {
		if err := mb.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (mb MissedBlock) Validate() error {
	if mb.Index < 0 {
		return fmt.Errorf("invalid missed block index. Index should be >= 0, got: %d", mb.Index)
	}
	return nil
}

func (vp VotingPowerFP) Validate() error {
	if vp.FpBtcPk == nil {
		return fmt.Errorf("invalid voting power. empty finality provider BTC public key")
	}

	if vp.FpBtcPk.Size() != types.BIP340PubKeyLen {
		return fmt.Errorf("invalid voting power. finality provider BTC public key length: got %d, want %d", vp.FpBtcPk.Size(), types.BIP340PubKeyLen)
	}

	return nil
}

func (vpc VotingPowerDistCacheBlkHeight) Validate() error {
	if vpc.VpDistribution == nil {
		return fmt.Errorf("invalid voting power distribution cache. empty voting power distribution")
	}

	return vpc.VpDistribution.Validate()
}

// Helper function to sort slices to get a deterministic
// result on the tests
func SortData(gs *GenesisState) {
	sort.Slice(gs.IndexedBlocks, func(i, j int) bool {
		return gs.IndexedBlocks[i].Height < gs.IndexedBlocks[j].Height
	})
	sort.Slice(gs.Evidences, func(i, j int) bool {
		return lessByPubKeyAndHeight(gs.Evidences[i].FpBtcPk, gs.Evidences[i].BlockHeight, gs.Evidences[j].FpBtcPk, gs.Evidences[j].BlockHeight)
	})
	sort.Slice(gs.VoteSigs, func(i, j int) bool {
		return lessByPubKeyAndHeight(gs.VoteSigs[i].FpBtcPk, gs.VoteSigs[i].BlockHeight, gs.VoteSigs[j].FpBtcPk, gs.VoteSigs[j].BlockHeight)
	})
	sort.Slice(gs.PublicRandomness, func(i, j int) bool {
		return lessByPubKeyAndHeight(gs.PublicRandomness[i].FpBtcPk, gs.PublicRandomness[i].BlockHeight, gs.PublicRandomness[j].FpBtcPk, gs.PublicRandomness[j].BlockHeight)
	})
	sort.Slice(gs.PubRandCommit, func(i, j int) bool {
		return gs.PubRandCommit[i].FpBtcPk.MarshalHex() < gs.PubRandCommit[j].FpBtcPk.MarshalHex()
	})
	sort.Slice(gs.SigningInfos, func(i, j int) bool {
		return gs.SigningInfos[i].FpBtcPk.MarshalHex() < gs.SigningInfos[j].FpBtcPk.MarshalHex()
	})
	sort.Slice(gs.MissedBlocks, func(i, j int) bool {
		return gs.MissedBlocks[i].FpBtcPk.MarshalHex() < gs.MissedBlocks[j].FpBtcPk.MarshalHex()
	})
	sort.Slice(gs.VotingPowers, func(i, j int) bool {
		return lessByPubKeyAndHeight(gs.VotingPowers[i].FpBtcPk, gs.VotingPowers[i].BlockHeight, gs.VotingPowers[j].FpBtcPk, gs.VotingPowers[j].BlockHeight)
	})
	sort.Slice(gs.VpDstCache, func(i, j int) bool {
		return gs.VpDstCache[i].BlockHeight < gs.VpDstCache[j].BlockHeight
	})
}

// lessByPubKeyAndHeight provides a deterministic comparator by
// the pubkey and height.
func lessByPubKeyAndHeight(pk1 *types.BIP340PubKey, h1 uint64, pk2 *types.BIP340PubKey, h2 uint64) bool {
	hex1 := pk1.MarshalHex()
	hex2 := pk2.MarshalHex()

	if hex1 != hex2 {
		return hex1 < hex2
	}
	return h1 < h2
}
