package datagen

import (
	"fmt"
	"math/rand"
	"time"

	"cosmossdk.io/math"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"

	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
)

func GenRandomIndexedHeader(r *rand.Rand) *types.IndexedHeader {
	return &types.IndexedHeader{
		ConsumerId:        fmt.Sprintf("chain-%s", GenRandomHexStr(r, 6)),
		Hash:              GenRandomByteArray(r, 32),
		BabylonHeaderHash: GenRandomByteArray(r, 32),
		BabylonTxHash:     GenRandomByteArray(r, 32),
	}
}

func GenRandomIndexedHeaderWithConsumerId(r *rand.Rand, consumerId string) *types.IndexedHeader {
	h := GenRandomIndexedHeader(r)
	h.ConsumerId = consumerId
	return h
}

func GenRandomIndexedHeaderWithConsumerAndEpoch(r *rand.Rand, consumerId string, epoch uint64) *types.IndexedHeader {
	now := time.Now().UTC()
	h := GenRandomIndexedHeaderWithConsumerId(r, consumerId)
	h.BabylonEpoch = epoch
	h.Time = &now
	return h
}

func GenRandomBTCChainSegment(r *rand.Rand) *types.BTCChainSegment {
	btcHeader := GenRandomBtcdHeader(r)
	blkHash := btcHeader.BlockHash()
	validHashBytes := bbntypes.NewBTCHeaderHashBytesFromChainhash(&blkHash)
	validHeaderBytes := bbntypes.NewBTCHeaderBytesFromBlockHeader(btcHeader)
	nonZeroWork := math.NewUint(RandomIntOtherThan(r, 0, 1000000))
	return &types.BTCChainSegment{
		BtcHeaders: []*btclctypes.BTCHeaderInfo{
			{
				Header: &validHeaderBytes,
				Hash:   &validHashBytes,
				Work:   &nonZeroWork,
			},
		},
	}
}

func GenRandomChainInfo(r *rand.Rand) *types.ChainInfo {
	consumerId := fmt.Sprintf("chain-%s", GenRandomHexStr(r, 20))
	return GenRandomChainInfoWithConsumerId(r, consumerId)
}

func GenRandomChainInfoWithConsumerId(r *rand.Rand, consumerId string) *types.ChainInfo {
	return &types.ChainInfo{
		ConsumerId:   consumerId,
		LatestHeader: GenRandomIndexedHeaderWithConsumerId(r, consumerId),
		LatestForks: &types.Forks{
			Headers: []*types.IndexedHeader{GenRandomIndexedHeaderWithConsumerId(r, consumerId)},
		},
	}
}

func GenRandomChainInfoWithProof(r *rand.Rand) *types.ChainInfoWithProof {
	return &types.ChainInfoWithProof{
		ChainInfo: GenRandomChainInfo(r),
		ProofHeaderInEpoch: &cmtcrypto.ProofOps{
			Ops: []cmtcrypto.ProofOp{
				cmtcrypto.ProofOp{},
			},
		},
	}
}

func GenRandomChainInfoWithProofAndConsumerId(r *rand.Rand, consumerId string) *types.ChainInfoWithProof {
	return &types.ChainInfoWithProof{
		ChainInfo: GenRandomChainInfoWithConsumerId(r, consumerId),
		ProofHeaderInEpoch: &cmtcrypto.ProofOps{
			Ops: []cmtcrypto.ProofOp{
				cmtcrypto.ProofOp{},
			},
		},
	}
}

func GenRandomProofEpochSealed(r *rand.Rand) *types.ProofEpochSealed {
	vs, _ := GenerateValidatorSetWithBLSPrivKeys(int(RandomIntOtherThan(r, 0, 20)))
	return &types.ProofEpochSealed{
		ValidatorSet: vs.ValSet,
		ProofEpochInfo: &cmtcrypto.ProofOps{
			Ops: []cmtcrypto.ProofOp{
				cmtcrypto.ProofOp{},
			},
		},
		ProofEpochValSet: &cmtcrypto.ProofOps{
			Ops: []cmtcrypto.ProofOp{
				cmtcrypto.ProofOp{},
			},
		},
	}
}

func GenRandomZoneconciergeGenState(r *rand.Rand) *types.GenesisState {
	var (
		entriesCount     = int(RandomIntOtherThan(r, 0, 20)) + 1 // make sure there's always at least one entry
		chainsInfo       = make([]*types.ChainInfo, entriesCount)
		idxHeaders       = make([]*types.IndexedHeader, entriesCount)
		forks            = make([]*types.Forks, entriesCount)
		chainsEpochsInfo = make([]*types.EpochChainInfoEntry, entriesCount)
		sealedEpochs     = make([]*types.SealedEpochProofEntry, entriesCount)
	)

	for i := range entriesCount {
		epochNum := uint64(i + 1)
		ci := GenRandomChainInfo(r)
		chainsInfo[i] = ci

		idxHeaders[i] = GenRandomIndexedHeaderWithConsumerAndEpoch(r, ci.ConsumerId, epochNum)

		forks[i] = &types.Forks{
			Headers: []*types.IndexedHeader{GenRandomIndexedHeaderWithConsumerAndEpoch(r, ci.ConsumerId, epochNum)},
		}

		chainsEpochsInfo[i] = &types.EpochChainInfoEntry{
			EpochNumber: epochNum,
			ChainInfo:   GenRandomChainInfoWithProofAndConsumerId(r, ci.ConsumerId),
		}

		sealedEpochs[i] = &types.SealedEpochProofEntry{
			EpochNumber: epochNum,
			Proof:       GenRandomProofEpochSealed(r),
		}
	}

	return types.NewGenesis(
		types.Params{
			IbcPacketTimeoutSeconds: RandomUInt32(r, 100000) + 1,
		},
		chainsInfo,
		idxHeaders,
		forks,
		chainsEpochsInfo,
		GenRandomBTCChainSegment(r),
		sealedEpochs,
	)
}
