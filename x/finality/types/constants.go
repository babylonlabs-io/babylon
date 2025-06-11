package types

const (
	// Setting max amount of finalized and rewarded blocks per EndBlock to 10000,
	// with this setting, block processing times are <1s, even if BTC finality
	// stalls a lot
	MaxFinalizedRewardedBlocksPerEndBlock = uint64(10000)
	// MaxPubRandCommitOffset defines the maximum number of blocks into the future
	// that a public randomness commitment start height can target. This limit prevents abuse by capping
	// the size of the commitments index, protecting against potential memory exhaustion
	// or performance degradation caused by excessive future commitments.
	MaxPubRandCommitOffset = 160_000
)
