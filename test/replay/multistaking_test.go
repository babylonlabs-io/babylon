package replay

import (
	"testing"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
)

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
