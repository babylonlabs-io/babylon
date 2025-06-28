package signingcontext

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	protocolName = "btcstaking"
	versionV0    = "0"
	fpPop        = "fp_pop"
	fpRandCommit = "fp_rand_commit"
	fpFinVote    = "fp_fin_vote"
	stakerPop    = "staker_pop"
)

func btcStakingV0Context(operationTag string, chainId string, address string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", protocolName, versionV0, operationTag, chainId, address)
}

// HashedHexContext returns the hex encoded sha256 hash of the context string i.e
// hex(sha256(contextString))
func HashedHexContext(contextString string) string {
	bytes := sha256.Sum256([]byte(contextString))
	return hex.EncodeToString(bytes[:])
}

// FpPopContextV0 returns context string in format:
// btcstaking/0/fp_pop/{chainId}/{address}
func FpPopContextV0(chainId string, address string) string {
	return HashedHexContext(btcStakingV0Context(fpPop, chainId, address))
}

// FpRandCommitContextV0 returns context string in format:
// btcstaking/0/fp_rand_commit/{chainId}/{address}
func FpRandCommitContextV0(chainId string, address string) string {
	return HashedHexContext(btcStakingV0Context(fpRandCommit, chainId, address))
}

// FpFinVoteContextV0 returns context string in format:
// btcstaking/0/fp_fin_vote/{chainId}/{address}
func FpFinVoteContextV0(chainId string, address string) string {
	return HashedHexContext(btcStakingV0Context(fpFinVote, chainId, address))
}

// StakerPopContextV0 returns context string in format:
// btcstaking/0/staker_pop/{chainId}/{address}
func StakerPopContextV0(chainId string, address string) string {
	return HashedHexContext(btcStakingV0Context(stakerPop, chainId, address))
}
