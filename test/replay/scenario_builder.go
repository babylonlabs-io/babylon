package replay

import "github.com/stretchr/testify/require"

type StandardScenario struct {
	driver            *BabylonAppDriver
	stakers           []*Staker
	finalityProviders []*FinalityProvider
	covenant          *CovenantSender
	activationHeight  uint64
}

func NewStandardScenario(driver *BabylonAppDriver) *StandardScenario {
	return &StandardScenario{
		driver: driver,
	}
}

// TODO: for now scenario works for small amount of fps and stakers, as with
// larger amound we are hitting per block gas limit, which leads to account sequence
// number errors. Improve this in the future.
func (s *StandardScenario) InitScenario(
	numFps int,
	delegationsPerFp int,
) {
	covSender := s.driver.CreateCovenantSender()
	fps := s.driver.CreateNFinalityProviderAccounts(numFps)
	// each staker will delegate to same fp
	stakers := s.driver.CreateNStakerAccounts(numFps)
	s.driver.GenerateNewBlockAssertExecutionSuccess()

	for _, fp := range fps {
		fp.RegisterFinalityProvider()
	}
	// register all fps in one block
	s.driver.GenerateNewBlockAssertExecutionSuccess()

	for _, fp := range fps {
		fp.CommitRandomness()
	}

	currnetEpochNunber := s.driver.GetEpoch().EpochNumber
	s.driver.ProgressTillFirstBlockTheNextEpoch()
	s.driver.FinializeCkptForEpoch(currnetEpochNunber)

	// commit all fps in one block
	s.driver.GenerateNewBlockAssertExecutionSuccess()

	for i, fp := range fps {
		for j := 0; j < delegationsPerFp; j++ {
			stakers[i].CreatePreApprovalDelegation(
				fp.BTCPublicKey(),
				// default values for now
				1000,
				100000000,
			)
		}
	}

	s.driver.GenerateNewBlockAssertExecutionSuccess()
	pendingDelegations := s.driver.GetPendingBTCDelegations(s.driver.t)
	require.Equal(s.driver.t, len(pendingDelegations), numFps*delegationsPerFp)

	covSender.SendCovenantSignatures()
	s.driver.GenerateNewBlockAssertExecutionSuccess()

	verifiedDelegations := s.driver.GetVerifiedBTCDelegations(s.driver.t)
	require.Equal(s.driver.t, len(verifiedDelegations), numFps*delegationsPerFp)

	s.driver.ActivateVerifiedDelegations(numFps * delegationsPerFp)
	s.driver.GenerateNewBlockAssertExecutionSuccess()

	activationHeight := s.driver.GetActivationHeight(s.driver.t)
	require.Greater(s.driver.t, activationHeight, uint64(0))

	activeFps := s.driver.GetActiveFpsAtHeight(s.driver.t, activationHeight)
	require.Equal(s.driver.t, len(activeFps), numFps)

	s.covenant = covSender
	s.stakers = stakers
	s.finalityProviders = fps
	s.activationHeight = activationHeight
}

func (s *StandardScenario) FinalityFinalizeBlocks(fromBlockToFinalize, numBlocksToFinalize uint64) uint64 {
	d := s.driver
	t := d.t

	latestFinalizedBlockHeight := uint64(0)
	for blkHeight := fromBlockToFinalize; blkHeight <= fromBlockToFinalize+numBlocksToFinalize; blkHeight++ {
		bl := d.GetIndexedBlock(blkHeight)
		require.Equal(t, bl.Finalized, false)

		for _, fp := range s.finalityProviders {
			fp.CastVote(blkHeight)
		}

		d.GenerateNewBlockAssertExecutionSuccess()

		bl = d.GetIndexedBlock(blkHeight)
		require.Equal(t, bl.Finalized, true)
		latestFinalizedBlockHeight = blkHeight
	}

	return latestFinalizedBlockHeight
}
