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
	ParamsKey           = []byte("p_btcstkconsumer") // ParamsKey stores module parameters (collections.Item[Params])
	ConsumerRegisterKey = []byte{0x01}               // ConsumerRegisterKey stores consumer registry per consumer ID (collections.Map[string, ConsumerRegister])
	FinalityContractKey = []byte{0x02}               // FinalityContractKey stores registered rollups finality contract addresses (collections.KeySet[string])
)

func KeyPrefix(p string) []byte {
	return []byte(p)
}
