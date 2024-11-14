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

// TestBTCStakingIntegrationTestSuite includes btc staking integration related tests
func TestBTCStakingIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(BTCStakingIntegrationTestSuite))
}

// TestBCDConsumerIntegrationTestSuite includes babylon<->bcd integration related tests
func TestBCDConsumerIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(BCDConsumerIntegrationTestSuite))
}

func TestBTCStakingPreApprovalTestSuite(t *testing.T) {
	suite.Run(t, new(BTCStakingPreApprovalTestSuite))
}

// TestSoftwareUpgradeV1TestnetTestSuite tests software upgrade of v1 testnet end-to-end
func TestSoftwareUpgradeV1TestnetTestSuite(t *testing.T) {
	suite.Run(t, new(SoftwareUpgradeV1TestnetTestSuite))
}
