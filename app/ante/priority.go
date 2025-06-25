package ante

import (
	"math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	btcctypes "github.com/babylonlabs-io/babylon/v3/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

const (
	// RegularTxMaxPriority is the max priority for transactions with regular messages.
	// Thus, the reserved priority range for protocol liveness-related messages is (RegularTxMaxPriority, MaxInt64]
	RegularTxMaxPriority = math.MaxInt64 - 1000
	// LivenessTxPriority is the priority for protocol liveness-related messages.
	// For the moment, the priority is the same for all of these messages
	LivenessTxPriority = RegularTxMaxPriority + 100
)

// PriorityDecorator assigns higher priority to protocol liveness-related transactions
type PriorityDecorator struct{}

func NewPriorityDecorator() PriorityDecorator {
	return PriorityDecorator{}
}

// Assigns higher priority to protocol liveness-related transactions
func (pd PriorityDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	// Cap priority for txs with regular messages
	// Use higher priorities for liveness-related txs
	priority := min(ctx.Priority(), RegularTxMaxPriority)

	if isLivenessTx(tx) {
		priority = LivenessTxPriority
	}

	newCtx := ctx.WithPriority(priority)

	return next(newCtx, tx, simulate)
}

// isLivenessTx returns true if ALL its messages are liveness-related
func isLivenessTx(tx sdk.Tx) bool {
	if len(tx.GetMsgs()) == 0 {
		return false
	}

	for _, msg := range tx.GetMsgs() {
		switch msg.(type) {
		case *btclctypes.MsgInsertHeaders, // BTC light client
			// BTC timestamping
			*btcctypes.MsgInsertBTCSpvProof,
			// BTC staking
			*bstypes.MsgAddCovenantSigs,
			*bstypes.MsgBTCUndelegate,
			*bstypes.MsgSelectiveSlashingEvidence,
			*bstypes.MsgAddBTCDelegationInclusionProof,
			// BTC staking finality
			*ftypes.MsgAddFinalitySig:
			continue
		default:
			return false
		}
	}
	return true
}
