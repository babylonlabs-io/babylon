package types

import (
	"bytes"
	"fmt"

	"github.com/babylonlabs-io/babylon/crypto/eots"
	"github.com/btcsuite/btcd/btcec/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
	}
}

// msgToSignForVote returns the message for an EOTS signature
// The EOTS signature on a block will be (blockHeight || blockHash)
func msgToSignForVote(blockHeight uint64, blockHash []byte) []byte {
	return append(sdk.Uint64ToBigEndian(blockHeight), blockHash...)
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

func (ib *IndexedBlock) MsgToSign() []byte {
	return msgToSignForVote(ib.Height, ib.AppHash)
}

func (e *Evidence) canonicalMsgToSign() []byte {
	return msgToSignForVote(e.BlockHeight, e.CanonicalAppHash)
}

func (e *Evidence) forkMsgToSign() []byte {
	return msgToSignForVote(e.BlockHeight, e.ForkAppHash)
}

func (e *Evidence) ValidateBasic() error {
	if e.FpBtcPk == nil {
		return fmt.Errorf("empty FpBtcPk")
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
func (e *Evidence) ExtractBTCSK() (*btcec.PrivateKey, error) {
	if !e.IsSlashable() {
		return nil, fmt.Errorf("the evidence lacks some fields so does not allow extracting BTC SK")
	}
	btcPK, err := e.FpBtcPk.ToBTCPK()
	if err != nil {
		return nil, err
	}
	return eots.Extract(
		btcPK, e.PubRand.ToFieldVal(),
		e.canonicalMsgToSign(), e.CanonicalFinalitySig.ToModNScalar(), // msg and sig for canonical block
		e.forkMsgToSign(), e.ForkFinalitySig.ToModNScalar(), // msg and sig for fork block
	)
}
