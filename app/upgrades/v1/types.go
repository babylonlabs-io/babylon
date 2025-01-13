package v1

type UpgradeDataString struct {
	BtcStakingParamsStr       string
	FinalityParamStr          string
	IncentiveParamStr         string
	CosmWasmParamStr          string
	NewBtcHeadersStr          string
	TokensDistributionStr     string
	AllowedStakingTxHashesStr string
}

type DataTokenDistribution struct {
	TokenDistribution []struct {
		AddressSender   string `json:"address_sender"`
		AddressReceiver string `json:"address_receiver"`
		Amount          int64  `json:"amount"`
	} `json:"token_distribution"`
}

type AllowedStakingTransactionHashes struct {
	TxHashes []string `json:"tx_hashes"`
}
