//go:build e2e
// +build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// IBCTransferTestSuite tests IBC transfer end-to-end
func TestIBCTransferTestSuite(t *testing.T) {
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

// TestSoftwareUpgradeV3TestSuite tests software upgrade from v2.2.0 to v3 end-to-end
func TestSoftwareUpgradeV3TestSuite(t *testing.T) {
	suite.Run(t, new(SoftwareUpgradeV3TestSuite))
}

// TestSoftwareUpgradeV3RC4TestSuite tests software upgrade from v3.0.0-rc3 to v3rc4 end-to-end
func TestSoftwareUpgradeV3RC4TestSuite(t *testing.T) {
	suite.Run(t, new(SoftwareUpgradeV3RC4TestSuite))
}

// TestFinalityContractTestSuite tests rollup finality contracts integration
func TestFinalityContractTestSuite(t *testing.T) {
	suite.Run(t, new(FinalityContractTestSuite))
}

// TestIbcCallbackBsnAddRewards tests BSN fee collection via IBC callbacks end-to-end
func TestIbcCallbackBsnAddRewards(t *testing.T) {
	suite.Run(t, new(IbcCallbackBsnAddRewards))
}

// TestBtcRewardsDistributionBsnRollup tests the bsn rewards for rollups
func TestBtcRewardsDistributionBsnRollup(t *testing.T) {
	suite.Run(t, new(BtcRewardsDistributionBsnRollup))
}

// TestBTCStakeExpansionTestSuite tests BTC stake expansion end-to-end
func TestBTCStakeExpansionTestSuite(t *testing.T) {
	suite.Run(t, new(BTCStakeExpansionTestSuite))
}
