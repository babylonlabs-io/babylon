package v1

type UpgradeDataString struct {
	BtcStakingParamStr    string
	FinalityParamStr      string
	CosmWasmParamStr      string
	NewBtcHeadersStr      string
	TokensDistributionStr string
}

type DataTokenDistribution struct {
	TokenDistribution []struct {
		AddressSender   string `json:"address_sender"`
		AddressReceiver string `json:"address_receiver"`
		Amount          int64  `json:"amount"`
	} `json:"token_distribution"`
}
