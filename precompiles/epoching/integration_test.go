package epoching_test

import (
	"github.com/stretchr/testify/require"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	//. "github.com/onsi/gomega"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/precompiles/epoching"
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

func (s *PrecompileIntegrationTestSuite) SetupTest() {
	s.BaseTestSuite.SetupApp(s.T())

	a, err := epoching.LoadABI()
	require.NoError(s.T(), err)
	s.abi = a
	s.addr = common.HexToAddress(epoching.EpochingPrecompileAddress)
}

func TestPrecompileIntegrationTestSuite(t *testing.T) {
	_ = Describe("Calling epoching precompile directly", func() {
		var s *PrecompileIntegrationTestSuite

		BeforeEach(func() {
			s.SetupTest()
		})
	})
}
