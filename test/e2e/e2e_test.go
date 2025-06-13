//go:build e2e
// +build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestVoteExtensionTestSuite(t *testing.T) {
	suite.Run(t, new(VoteExtensionTestSuite))
}

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

// TestSoftwareUpgradeV1TestnetTestSuite tests software upgrade of v1 testnet end-to-end
func TestSoftwareUpgradeV1TestnetTestSuite(t *testing.T) {
	suite.Run(t, new(SoftwareUpgradeV1TestnetTestSuite))
}
