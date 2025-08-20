package feemarketwrapper

// NOTE: each prefix value should be different from the transient store key prefixes in evm/x/feemarket/types/keys.go
const (
	prefixTransientRefundableGasWantedKey = iota + 2
	prefixTransientRefundableGasUsedKey
)

var (
	KeyPrefixRefundableGasWanted = []byte{prefixTransientRefundableGasWantedKey}
	KeyPrefixRefundableGasUsed   = []byte{prefixTransientRefundableGasUsedKey}
)
