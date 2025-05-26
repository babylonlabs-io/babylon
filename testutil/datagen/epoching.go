package datagen

import (
	"math/rand"
	"time"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// getFirstBlockHeight returns the height of the first block of a given epoch and epoch interval
// NOTE: this is only a function for testing and assumes static epoch interval
func getFirstBlockHeight(epochNumber uint64, epochInterval uint64) uint64 {
	if epochNumber == 0 {
		return 0
	} else {
		return (epochNumber-1)*epochInterval + 1
	}
}

func GenRandomEpochNum(r *rand.Rand) uint64 {
	epochNum := r.Int63n(100)
	return uint64(epochNum)
}

func GenRandomEpochInterval(r *rand.Rand) uint64 {
	epochInterval := r.Int63n(10) + 2 // interval should be at least 2
	return uint64(epochInterval)
}

func GenRandomEpoch(r *rand.Rand) *epochingtypes.Epoch {
	epochNum := GenRandomEpochNum(r)
	epochInterval := GenRandomEpochInterval(r)
	firstBlockHeight := getFirstBlockHeight(epochNum, epochInterval)
	lastBlockHeader := GenRandomTMHeader(r, "test-chain", firstBlockHeight+epochInterval-1)
	epoch := epochingtypes.NewEpoch(
		epochNum,
		epochInterval,
		firstBlockHeight,
		&lastBlockHeader.Time,
	)
	sealerHeader := GenRandomTMHeader(r, "test-chain", firstBlockHeight+epochInterval+1) // 2nd block in the next epoch
	epoch.SealerBlockHash = GenRandomBlockHash(r)
	epoch.SealerAppHash = sealerHeader.AppHash
	return &epoch
}

func GenRandomEpochingGenesisState(r *rand.Rand) *epochingtypes.GenesisState {
	var (
		entriesCount   = int(RandomIntOtherThan(r, 0, 20)) + 1 // make sure there's always at least one entry
		now            = time.Now().UTC()
		vs             = GenRandomValSet(entriesCount)
		slashedVs      = GenRandomValSet(entriesCount)
		epochs         = make([]*epochingtypes.Epoch, entriesCount)
		qs             = make([]*epochingtypes.EpochQueue, entriesCount)
		valSets        = make([]*epochingtypes.EpochValidatorSet, entriesCount)
		slashedValSets = make([]*epochingtypes.EpochValidatorSet, entriesCount)
		valsLc         = make([]*epochingtypes.ValidatorLifecycle, entriesCount)
		delsLc         = make([]*epochingtypes.DelegationLifecycle, entriesCount)
	)

	for i := range entriesCount {
		// populate epochs
		epochNum := uint64(i) + 1
		epochs[i] = GenRandomEpoch(r)
		epochs[i].EpochNumber = epochNum
		epochs[i].FirstBlockHeight = epochNum + 1000
		epochs[i].SealerAppHash = append(epochs[i].SealerAppHash, byte(epochNum))

		// queued message
		msgCreateVal, err := stakingtypes.NewMsgCreateValidator(
			GenRandomValidatorAddress().String(),
			ed25519.GenPrivKey().PubKey(),
			sdk.NewInt64Coin(appparams.DefaultBondDenom, 1000),
			stakingtypes.Description{},
			stakingtypes.NewCommissionRates(math.LegacyZeroDec(), math.LegacyZeroDec(), math.LegacyZeroDec()),
			math.OneInt(),
		)
		if err != nil {
			panic(err)
		}

		qMsg, err := epochingtypes.NewQueuedMessage(1, now, []byte("tx id 1"), msgCreateVal)
		if err != nil {
			panic(err)
		}
		qs[i] = &epochingtypes.EpochQueue{
			EpochNumber: epochNum,
			Msgs:        []*epochingtypes.QueuedMessage{&qMsg},
		}

		// epochs validator set
		valSets[i] = &epochingtypes.EpochValidatorSet{
			EpochNumber: epochNum,
			Validators:  make([]*epochingtypes.Validator, entriesCount),
		}

		for j, v := range vs {
			valSets[i].Validators[j] = &v
		}

		slashedValSets[i] = &epochingtypes.EpochValidatorSet{
			EpochNumber: epochNum,
			Validators:  make([]*epochingtypes.Validator, entriesCount),
		}

		for j, v := range slashedVs {
			slashedValSets[i].Validators[j] = &v
		}

		valsLc[i] = &epochingtypes.ValidatorLifecycle{
			ValAddr: GenRandomValidatorAddress().String(),
			ValLife: []*epochingtypes.ValStateUpdate{{}},
		}

		delsLc[i] = &epochingtypes.DelegationLifecycle{
			DelAddr: GenRandomAddress().String(),
			DelLife: []*epochingtypes.DelegationStateUpdate{{}},
		}
	}

	return &epochingtypes.GenesisState{
		Params:               epochingtypes.DefaultParams(),
		Epochs:               epochs,
		Queues:               qs,
		ValidatorSets:        valSets,
		SlashedValidatorSets: slashedValSets,
		ValidatorsLifecycle:  valsLc,
		DelegationsLifecycle: delsLc,
	}
}
