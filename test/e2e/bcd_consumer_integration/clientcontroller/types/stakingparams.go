package types

import (
	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
)

type StakingParams struct {
	// K-deep
	ComfirmationTimeBlocks uint32
	// W-deep
	FinalizationTimeoutBlocks uint32

	// Minimum amount of tx fee (quantified in Satoshi) needed for the pre-signed slashing tx
	MinSlashingTxFeeSat btcutil.Amount

	// Bitcoin public keys of the covenant committee
	CovenantPks []*btcec.PublicKey

	// Address to which slashing transactions are sent
	SlashingPkScript []byte

	// Minimum number of signatures needed for the covenant multisignature
	CovenantQuorum uint32

	// The staked amount to be slashed, expressed as a decimal (e.g., 0.5 for 50%).
	SlashingRate sdkmath.LegacyDec

	// The minimum time for unbonding transaction timelock in BTC blocks
	MinUnbondingTime uint32
}

// MinimumUnbondingTime returns the minimum unbonding time. It is the bigger value from:
// - MinUnbondingTime
// - CheckpointFinalizationTimeout
func (p *StakingParams) MinimumUnbondingTime() uint32 {
	return sdkmath.Max[uint32](
		p.MinUnbondingTime,
		p.FinalizationTimeoutBlocks,
	)
}
