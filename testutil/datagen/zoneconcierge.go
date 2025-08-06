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

func GenRandomIndexedHeaderWithProofAndConsumerId(r *rand.Rand, consumerId string) *types.IndexedHeaderWithProof {
	return &types.IndexedHeaderWithProof{
		Header: GenRandomIndexedHeaderWithConsumerId(r, consumerId),
		Proof: &cmtcrypto.ProofOps{
			Ops: []cmtcrypto.ProofOp{
				cmtcrypto.ProofOp{},
			},
		},
	}
}

func GenRandomBSNBTCState(r *rand.Rand) *types.BSNBTCState {
	return &types.BSNBTCState{
		LastSentSegment: GenRandomBTCChainSegment(r),
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
		finalizedHeaders = make([]*types.FinalizedHeaderEntry, entriesCount)
		bsnBTCStates     = make([]*types.BSNBTCStateEntry, entriesCount)
		sealedEpochs     = make([]*types.SealedEpochProofEntry, entriesCount)
	)

	for i := range entriesCount {
		epochNum := uint64(i + 1)
		consumerId := fmt.Sprintf("chain-%s", GenRandomHexStr(r, 20))

		finalizedHeaders[i] = &types.FinalizedHeaderEntry{
			EpochNumber:     epochNum,
			ConsumerId:      consumerId,
			HeaderWithProof: GenRandomIndexedHeaderWithProofAndConsumerId(r, consumerId),
		}

		bsnBTCStates[i] = &types.BSNBTCStateEntry{
			ConsumerId: consumerId,
			State:      GenRandomBSNBTCState(r),
		}

		sealedEpochs[i] = &types.SealedEpochProofEntry{
			EpochNumber: epochNum,
			Proof:       GenRandomProofEpochSealed(r),
		}
	}

	return &types.GenesisState{
		Params: types.Params{
			IbcPacketTimeoutSeconds: RandomUInt32(r, 100000) + 1,
		},
		PortId:             types.PortID,
		FinalizedHeaders:   finalizedHeaders,
		LastSentSegment:    GenRandomBTCChainSegment(r),
		SealedEpochsProofs: sealedEpochs,
		BsnBtcStates:       bsnBTCStates,
	}
}
