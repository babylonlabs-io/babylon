package feemarketwrapper

// NOTE: Use high prefix values (200+) to avoid collision with cosmos-evm feemarket module
// current cosmos-evm feemarket uses: 1 (prefixTransientBlockGasUsed)
// reserve 200+ range for babylon feemarket wrapper extensions
const (
	prefixTransientRefundableGasWantedKey = iota + 200
	prefixTransientRefundableGasUsedKey
)

var (
	KeyPrefixRefundableGasWanted = []byte{prefixTransientRefundableGasWantedKey}
	KeyPrefixRefundableGasUsed   = []byte{prefixTransientRefundableGasUsedKey}
)
