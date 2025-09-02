package epoching_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/test/integration/precompiles"
)

type PrecompileIntegrationTestSuite struct {
	suite.Suite
	precompiles.BaseTestSuite

	abi  abi.ABI
	addr common.Address
}

func TestPrecompileTestSuite(t *testing.T) {
	suite.Run(t, new(PrecompileIntegrationTestSuite))
}
