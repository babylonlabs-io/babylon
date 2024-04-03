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
	ParamsKey                        = []byte("p_btcstkconsumer")
	ConsumerChainRegisterKey         = []byte{0x01} // ConsumerChainRegisterKey defines the key to the chain register for each CZ in store
	ConsumerFinalityProviderKey      = []byte{0x02} // ConsumerFinalityProviderKey defines the key to the CZ finality providers store
	ConsumerFinalityProviderChainKey = []byte{0x03} // ConsumerFinalityProviderChainKey defines the key to the CZ chains per FP BTC PK store
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
