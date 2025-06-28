package types

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v2/crypto/eots"
	"github.com/babylonlabs-io/babylon/v2/types"
)

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
