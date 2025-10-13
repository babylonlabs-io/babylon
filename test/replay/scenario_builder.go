package replay

import (
	sdkmath "cosmossdk.io/math"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

const (
	defaultStakingTime = uint32(1000)
)

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
	validators, err := s.driver.App.StakingKeeper.GetAllValidators(s.driver.Ctx())
	require.NoError(s.driver.t, err)
	val := validators[0]
	valAddr := sdk.MustValAddressFromBech32(val.OperatorAddress)

	covSender := s.driver.CreateCovenantSender()
	fps := s.driver.CreateNFinalityProviderAccounts(numFps)
	// each staker will delegate to same fp
	stakers := s.driver.CreateNStakerAccounts(numFps)

	for _, del := range stakers {
		delAmt := sdkmath.NewInt(20_000000)
		s.driver.TxWrappedDelegate(del.SenderInfo, valAddr.String(), delAmt)
	}

	s.driver.GenerateNewBlockAssertExecutionSuccess()

	oldEpochNumber := s.driver.GetEpoch().EpochNumber
	s.driver.ProgressTillFirstBlockTheNextEpoch()
	s.driver.FinalizeCkptForEpoch(oldEpochNumber)

	for _, fp := range fps {
		fp.RegisterFinalityProvider()
	}
	// register all fps in one block
	s.driver.GenerateNewBlockAssertExecutionSuccess()

	for _, fp := range fps {
		fp.CommitRandomness()
	}

	currentEpochNumber := s.driver.GetEpoch().EpochNumber
	s.driver.ProgressTillFirstBlockTheNextEpoch()
	s.driver.FinalizeCkptForEpoch(currentEpochNumber)

	// commit all fps in one block
	s.driver.GenerateNewBlockAssertExecutionSuccess()

	for i, fp := range fps {
		for j := 0; j < delegationsPerFp; j++ {
			stakers[i].CreatePreApprovalDelegation(
				[]*bbn.BIP340PubKey{fp.BTCPublicKey()},
				defaultStakingTime,
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
	require.GreaterOrEqual(s.driver.t, numFps, len(activeFps))

	s.covenant = covSender
	s.stakers = stakers
	s.finalityProviders = fps
	s.activationHeight = activationHeight
}

func (s *StandardScenario) CreateActiveBtcDel(fp *FinalityProvider, staker *Staker, totalSat int64) {
	staker.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{fp.BTCPublicKey()},
		defaultStakingTime,
		totalSat,
	)

	s.driver.GenerateNewBlockAssertExecutionSuccess()

	s.covenant.SendCovenantSignatures()
	s.driver.GenerateNewBlockAssertExecutionSuccess()

	s.driver.ActivateVerifiedDelegations(1)
	s.driver.GenerateNewBlockAssertExecutionSuccess()
}

func (s *StandardScenario) FinalityFinalizeBlocksAllVotes(fromBlockToFinalize, numBlocksToFinalize uint64) uint64 {
	return s.FinalityFinalizeBlocks(fromBlockToFinalize, numBlocksToFinalize, s.FpMapBtcPkHex())
}

func (s *StandardScenario) FpMapBtcPkHex() map[string]struct{} {
	return s.FpMapBtcPkHexQnt(len(s.finalityProviders))
}

func (s *StandardScenario) FpMapBtcPkHexQnt(limit int) map[string]struct{} {
	fpsToVote := make(map[string]struct{}, limit)
	for i, fp := range s.finalityProviders {
		fpsToVote[fp.BTCPublicKey().MarshalHex()] = struct{}{}
		if i >= limit {
			return fpsToVote
		}
	}
	return fpsToVote
}

func (s *StandardScenario) FinalityFinalizeBlocks(fromBlockToFinalize, numBlocksToFinalize uint64, fpsToVote map[string]struct{}) uint64 {
	d := s.driver
	t := d.t

	latestFinalizedBlockHeight := uint64(0)
	for blkHeight := fromBlockToFinalize; blkHeight <= fromBlockToFinalize+numBlocksToFinalize; blkHeight++ {
		bl := d.GetIndexedBlock(blkHeight)
		require.Equal(t, bl.Finalized, false)

		s.FinalityCastVotes(blkHeight, fpsToVote)

		d.GenerateNewBlockAssertExecutionSuccess()

		bl = d.GetIndexedBlock(blkHeight)
		require.Equal(t, bl.Finalized, true)
		latestFinalizedBlockHeight = blkHeight
	}

	return latestFinalizedBlockHeight
}

func (s *StandardScenario) FinalityCastVotes(blkHeight uint64, fpsToVote map[string]struct{}) {
	d := s.driver

	for _, fp := range s.finalityProviders {
		fpPk := fp.BTCPublicKey().MarshalHex()
		_, shouldVote := fpsToVote[fpPk]
		if !shouldVote {
			continue
		}

		vp := d.App.FinalityKeeper.GetVotingPower(d.Ctx(), *fp.BTCPublicKey(), blkHeight)
		if vp <= 0 {
			continue
		}

		fp.CastVote(blkHeight)
	}
}

func (s *StandardScenario) StakersAddr() []sdk.AccAddress {
	v := make([]sdk.AccAddress, len(s.stakers))
	for i, staker := range s.stakers {
		v[i] = staker.Address()
	}
	return v
}
