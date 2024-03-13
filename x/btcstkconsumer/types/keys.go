package types

const (
	// ModuleName defines the module name
	ModuleName = "btcstkconsumer"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_btcstkconsumer"

	// QuerierRoute defines the module's query routing key
	QuerierRoute = ModuleName
)

var (
	ParamsKey        = []byte("p_btcstkconsumer")
	ChainRegisterKey = []byte{0x01} // ChainRegisterKey defines the key to store the chain register for each CZ in store
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
