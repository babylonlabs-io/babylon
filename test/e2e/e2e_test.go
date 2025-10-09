//go:build e2e
// +build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// IBCTransferTestSuite tests IBC transfer end-to-end
func TestIBCTranferTestSuite(t *testing.T) {
	suite.Run(t, new(IBCTransferTestSuite))
}

// TestBTCTimestampingTestSuite tests BTC timestamping protocol end-to-end
func TestBTCTimestampingTestSuite(t *testing.T) {
	suite.Run(t, new(BTCTimestampingTestSuite))
}

// TestBTCStakingTestSuite tests BTC staking protocol end-to-end
func TestBTCStakingTestSuite(t *testing.T) {
	suite.Run(t, new(BTCStakingTestSuite))
}

// TestBTCRewardsDistribution tests BTC staking rewards distribution end-to-end
// that involves x/btcstaking, x/finality, x/incentives and x/mint to give out rewards.
func TestBTCRewardsDistribution(t *testing.T) {
	suite.Run(t, new(BtcRewardsDistribution))
}

// TestGovResumeFinality tests resume of finality voting gov prop
func TestGovResumeFinality(t *testing.T) {
	suite.Run(t, new(GovFinalityResume))
}

func TestBTCStakingPreApprovalTestSuite(t *testing.T) {
	suite.Run(t, new(BTCStakingPreApprovalTestSuite))
}

// ICATestSuite tests ICA end-to-end
func TestICATestSuite(t *testing.T) {
	suite.Run(t, new(ICATestSuite))
}

// TestSoftwareUpgradeV23To4TestSuite tests software upgrade from v2.3 to v4 end-to-end
func TestSoftwareUpgradeV23To4TestSuite(t *testing.T) {
	suite.Run(t, new(SoftwareUpgradeV23To4TestSuite))
}

// TestEpochingSpamPreventionTestSuite tests epoching spam prevention end-to-end
func TestEpochingSpamPreventionTestSuite(t *testing.T) {
	suite.Run(t, new(EpochingSpamPreventionTestSuite))
}

// TestBTCStakeExpansionTestSuite tests BTC stake expansion end-to-end
func TestBTCStakeExpansionTestSuite(t *testing.T) {
	suite.Run(t, new(BTCStakeExpansionTestSuite))
}

// TestValidatorJailingTestSuite tests validator jailing scenario end-to-end
func TestValidatorJailingTestSuite(t *testing.T) {
	suite.Run(t, new(ValidatorJailingTestSuite))
}
