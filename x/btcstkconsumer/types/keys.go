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
	ParamsKey                = []byte("p_btcstkconsumer")
	ChainRegisterKey         = []byte{0x01} // ChainRegisterKey defines the key to the chain register for each CZ in store
	FinalityProviderKey      = []byte{0x02} // FinalityProviderKey defines the key to the CZ finality providers store
	FinalityProviderChainKey = []byte{0x03} // FinalityProviderChainKey defines the key to the CZ chains per FP BTC PK store
	BTCDelegatorKey          = []byte{0x04} // key prefix for the BTC delegators
	BTCDelegationKey         = []byte{0x05} // key prefix for the BTC delegations
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
