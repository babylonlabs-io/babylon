package types

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/crypto/eots"
	"github.com/babylonlabs-io/babylon/v3/types"
	btcstktypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

// FinalityProviderState defines the possible states of a finality provider
// It is used in the power distribution change process
// to determine the state of each finality provider and how it affects the power distribution
type FinalityProviderState int32

const (
	// FinalityProviderState_UNKNOWN indicates the finality provider state is unknown or uninitialized
	FinalityProviderState_UNKNOWN FinalityProviderState = iota
	// FinalityProviderState_UNJAILED indicates the finality provider is active and can participate
	FinalityProviderState_UNJAILED
	// FinalityProviderState_JAILED indicates the finality provider is jailed and cannot participate
	FinalityProviderState_JAILED
	// FinalityProviderState_SLASHED indicates the finality provider has been slashed
	FinalityProviderState_SLASHED
)

// Processing state during the power distribution change process
// It holds the state of finality providers, BTC delegations, and events
// It is used to track the changes in the finality providers' states and the BTC delegations
// It is also used to determine the final state of each finality provider after the power distribution change
// The state is updated during the power distribution change process and is used to generate the
// final power distribution cache
type ProcessingState struct {
	// FPStatesByBtcPk is a map of the finality providers' state
	FPStatesByBtcPk map[string]FinalityProviderState
	// FpByBtcPk is a map where key is finality provider's BTC PK hex and value is the finality provider
	// It is used as cache to avoid fetching the finality provider from the store
	// during the power distribution change process
	FpByBtcPk map[string]*btcstktypes.FinalityProvider
	// DeltaSatsByFpBtcPk is a map where key is finality provider's BTC PK hex and value is the
	// delta of BTC delegations satoshis that were added or removed from the provider
	// during the power distribution change process
	DeltaSatsByFpBtcPk map[string]int64
	// A slice of the BTC delegations expired events
	ExpiredEvents []*btcstktypes.EventPowerDistUpdate_BtcDelStateUpdate
	// A slice of the slashed finality provider events
	SlashedEvents []*btcstktypes.EventPowerDistUpdate_SlashedFp
}

func NewProcessingState() *ProcessingState {
	return &ProcessingState{
		FPStatesByBtcPk:    map[string]FinalityProviderState{},
		FpByBtcPk:          map[string]*btcstktypes.FinalityProvider{},
		DeltaSatsByFpBtcPk: map[string]int64{},
		ExpiredEvents:      []*btcstktypes.EventPowerDistUpdate_BtcDelStateUpdate{},
		SlashedEvents:      []*btcstktypes.EventPowerDistUpdate_SlashedFp{},
	}
}

func (c *PubRandCommit) IsInRange(height uint64) bool {
	start, end := c.Range()
	return start <= height && height <= end
}

func (c *PubRandCommit) GetIndex(height uint64) (uint64, error) {
	start, end := c.Range()
	if start <= height && height <= end {
		return height - start, nil
	}
	return 0, ErrPubRandNotFound.Wrapf("the given height (%d) is not in range [%d, %d]", height, start, end)
}

func (c *PubRandCommit) EndHeight() uint64 {
	return c.StartHeight + c.NumPubRand - 1
}

// Range() returns the range of the heights that a public randomness is committed
// both values are inclusive
func (c *PubRandCommit) Range() (uint64, uint64) {
	return c.StartHeight, c.EndHeight()
}

func (c *PubRandCommit) ToResponse() *PubRandCommitResponse {
	return &PubRandCommitResponse{
		NumPubRand: c.NumPubRand,
		Commitment: c.Commitment,
		EpochNum:   c.EpochNum,
	}
}

func (c *PubRandCommit) Validate() error {
	if len(c.Commitment) == 0 {
		return ErrInvalidPubRandCommit.Wrap("empty commitment")
	}
	return nil
}

// msgToSignForVote returns the message for an EOTS signature
// The EOTS signature on a block will be (context || blockHeight || blockHash)
func msgToSignForVote(
	context string,
	blockHeight uint64,
	blockHash []byte,
) []byte {
	if len(context) == 0 {
		return append(sdk.Uint64ToBigEndian(blockHeight), blockHash...)
	}

	return append([]byte(context), append(sdk.Uint64ToBigEndian(blockHeight), blockHash...)...)
}

func (ib *IndexedBlock) Equal(ib2 *IndexedBlock) bool {
	if !bytes.Equal(ib.AppHash, ib2.AppHash) {
		return false
	}
	if ib.Height != ib2.Height {
		return false
	}
	// NOTE: we don't compare finalisation status here
	return true
}

func (ib IndexedBlock) Validate() error {
	if ib.Height > 0 && len(ib.AppHash) == 0 {
		return fmt.Errorf("invalid indexed block. Empty app hash")
	}
	return nil
}

func (e *Evidence) canonicalMsgToSign(context string) []byte {
	return msgToSignForVote(context, e.BlockHeight, e.CanonicalAppHash)
}

func (e *Evidence) forkMsgToSign(context string) []byte {
	return msgToSignForVote(context, e.BlockHeight, e.ForkAppHash)
}

func (e *Evidence) ValidateBasic() error {
	if e.FpBtcPk == nil {
		return fmt.Errorf("empty FpBtcPk")
	}
	if _, err := e.FpBtcPk.ToBTCPK(); err != nil {
		return err
	}
	if e.PubRand == nil {
		return fmt.Errorf("empty PubRand")
	}
	if len(e.CanonicalAppHash) != 32 {
		return fmt.Errorf("malformed CanonicalAppHash")
	}
	if len(e.ForkAppHash) != 32 {
		return fmt.Errorf("malformed ForkAppHash")
	}
	if e.ForkFinalitySig == nil {
		return fmt.Errorf("empty ForkFinalitySig")
	}
	if e.ForkFinalitySig.Size() != types.SchnorrEOTSSigLen {
		return fmt.Errorf("malformed ForkFinalitySig")
	}

	return nil
}

func (e *Evidence) IsSlashable() bool {
	if err := e.ValidateBasic(); err != nil {
		return false
	}
	if e.CanonicalFinalitySig == nil {
		return false
	}
	return true
}

// ExtractBTCSK extracts the BTC SK given the data in the evidence
// It is up to the caller to pass correct context string for the given chain and height
func (e *Evidence) ExtractBTCSK() (*btcec.PrivateKey, error) {
	if !e.IsSlashable() {
		return nil, fmt.Errorf("the evidence lacks some fields so does not allow extracting BTC SK")
	}
	btcPK, err := e.FpBtcPk.ToBTCPK()
	if err != nil {
		return nil, err
	}
	return eots.Extract(
		btcPK, e.PubRand.ToFieldValNormalized(),
		e.canonicalMsgToSign(e.SigningContext), e.CanonicalFinalitySig.ToModNScalar(), // msg and sig for canonical block
		e.forkMsgToSign(e.SigningContext), e.ForkFinalitySig.ToModNScalar(), // msg and sig for fork block
	)
}
