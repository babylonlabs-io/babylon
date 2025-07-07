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
	ParamsKey           = []byte("p_btcstkconsumer")
	ConsumerRegisterKey = []byte{0x01} // ConsumerRegisterKey defines the key to the chain register for each consumer in store
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
