package replay

import (
	"testing"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	abci "github.com/cometbft/cometbft/abci/types"
)

// packVerifiedDelegations packs activation of verified delegations into a single block
// with proper gas limits for each message
// It obeys all gas limits of the Babylon Genesis:
// - Every tx is less than 10M gas
// - Block will have less than 300M gas
func (d *BabylonAppDriver) packVerifiedDelegations() []*abci.ExecTxResult {
	block, _ := d.IncludeVerifiedStakingTxInBTC(0)
	activationMsgs := blockWithProofsToActivationMessages(block, d.GetDriverAccountAddress())

	for i, msg := range activationMsgs {
		var gaslimit uint64

		switch {
		case i < 5:
			gaslimit = 1_100_000
		case i < 10:
			gaslimit = 2_000_000
		case i < 15:
			gaslimit = 2_700_000
		case i < 20:
			gaslimit = 3_500_000
		case i < 25:
			gaslimit = 4_400_000
		case i < 30:
			gaslimit = 5_100_000
		case i < 35:
			gaslimit = 6_000_000
		case i < 40:
			gaslimit = 7_000_000
		case i < 45:
			gaslimit = 7_500_000
		case i < 50:
			gaslimit = 8_500_000
		default:
			gaslimit = 10_000_000
		}

		d.SendTxWithMessagesSuccess(d.t, d.SenderInfo, gaslimit, defaultFeeCoin, msg)
		d.IncSeq()
	}

	return d.GenerateNewBlockReturnResults()
}

func (driver *BabylonAppDriver) SendAndVerifyNDelegations(
	t *testing.T,
	staker *Staker,
	covSender *CovenantSender,
	keys []*bbn.BIP340PubKey,
	n int,
) {
	for i := 0; i < n; i++ {
		staker.CreatePreApprovalDelegation(
			keys,
			1000,
			100000000,
		)
	}

	driver.GenerateNewBlockAssertExecutionSuccess()
	covSender.SendCovenantSignatures()
	driver.GenerateNewBlockAssertExecutionSuccess()
}
