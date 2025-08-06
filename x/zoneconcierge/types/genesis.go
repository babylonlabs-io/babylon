package types

import (
	"errors"
	"sort"
	"strconv"

	"github.com/babylonlabs-io/babylon/v3/types"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// NewGenesis creates a new GenesisState instance
func NewGenesis(
	params Params,
	finalizedHeaders []*FinalizedHeaderEntry,
	lastSentSegment *BTCChainSegment,
	sealedEpochs []*SealedEpochProofEntry,
	bsnBtcStates []*BSNBTCStateEntry,
) *GenesisState {
	return &GenesisState{
		Params:             params,
		FinalizedHeaders:   finalizedHeaders,
		LastSentSegment:    lastSentSegment,
		SealedEpochsProofs: sealedEpochs,
		BsnBtcStates:       bsnBtcStates,
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := types.ValidateEntries(
		gs.FinalizedHeaders,
		func(fh *FinalizedHeaderEntry) string {
			// unique key is consumer id + epoch number
			return fh.ConsumerId + strconv.FormatUint(fh.EpochNumber, 10)
		}); err != nil {
		return err
	}

	if gs.LastSentSegment != nil {
		if err := gs.LastSentSegment.Validate(); err != nil {
			return err
		}
	}

	if err := types.ValidateEntries(gs.SealedEpochsProofs, func(se *SealedEpochProofEntry) uint64 { return se.EpochNumber }); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.BsnBtcStates, func(cs *BSNBTCStateEntry) string { return cs.ConsumerId }); err != nil {
		return err
	}

	return gs.Params.Validate()
}

func (fhe FinalizedHeaderEntry) Validate() error {
	if fhe.HeaderWithProof == nil {
		return errors.New("invalid finalized header entry. empty header with proof")
	}
	return fhe.HeaderWithProof.Validate()
}

func (cse BSNBTCStateEntry) Validate() error {
	if cse.State == nil {
		return errors.New("invalid BSN BTC state entry. empty state")
	}
	return cse.State.Validate()
}

func (sep SealedEpochProofEntry) Validate() error {
	if sep.Proof == nil {
		return errors.New("invalid sealed epoch entry. empty proof")
	}
	return sep.Proof.ValidateBasic()
}

// Helper function to sort slices to get a deterministic
// result on the tests
func SortData(gs *GenesisState) {
	sort.Slice(gs.FinalizedHeaders, func(i, j int) bool {
		if gs.FinalizedHeaders[i].EpochNumber != gs.FinalizedHeaders[j].EpochNumber {
			return gs.FinalizedHeaders[i].EpochNumber < gs.FinalizedHeaders[j].EpochNumber
		}
		return gs.FinalizedHeaders[i].ConsumerId < gs.FinalizedHeaders[j].ConsumerId
	})

	sort.Slice(gs.BsnBtcStates, func(i, j int) bool {
		return gs.BsnBtcStates[i].ConsumerId < gs.BsnBtcStates[j].ConsumerId
	})

	sort.Slice(gs.SealedEpochsProofs, func(i, j int) bool {
		if gs.SealedEpochsProofs[i].EpochNumber != gs.SealedEpochsProofs[j].EpochNumber {
			return gs.SealedEpochsProofs[i].EpochNumber < gs.SealedEpochsProofs[j].EpochNumber
		}
		return gs.SealedEpochsProofs[i].Proof.String() < gs.SealedEpochsProofs[j].Proof.String()
	})
}
