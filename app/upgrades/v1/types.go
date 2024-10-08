package v1

type UpgradeDataString struct {
	BtcStakingParamStr    string
	FinalityParamStr      string
	CosmWasmParamStr      string
	NewBtcHeadersStr      string
	SignedFPsStr          string
	TokensDistributionStr string
}

type DataSignedFps struct {
	SignedTxsFP []any `json:"signed_txs_create_fp"`
}

type DataTokenDistribution struct {
	TokenDistribution []struct {
		AddressSender   string `json:"address_sender"`
		AddressReceiver string `json:"address_receiver"`
		Amount          int64  `json:"amount"`
	} `json:"token_distribution"`
}
