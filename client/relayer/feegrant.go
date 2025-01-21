package relayerclient

// GetTxFeeGrant Get the feegrant params to use for the next TX. If feegrants are not configured for the chain client, the default key will be used for TX signing.
// Otherwise, a configured feegrantee will be chosen for TX signing in round-robin fashion.
func (cc *CosmosProvider) GetTxFeeGrant() (string, string) {
	// Default values
	txSignerKey := cc.PCfg.Key
	feeGranterKeyOrAddr := ""

	if cc.PCfg.FeeGrants == nil {
		// Fee grants are not configured; use the default key.
		return txSignerKey, feeGranterKeyOrAddr
	}

	// Use the configured Feegranter key for the next TX
	feeGranterKeyOrAddr = cc.PCfg.FeeGrants.GranterKeyOrAddr

	// If the Feegrant configuration has never been verified on chain, avoid using Feegrants.
	if feeGranterKeyOrAddr == "" || cc.PCfg.FeeGrants.BlockHeightVerified <= 0 {
		return txSignerKey, ""
	}

	// Pick the next managed grantee in the list as the TX signer
	lastGranteeIdx := cc.PCfg.FeeGrants.GranteeLastSignerIndex
	if lastGranteeIdx >= 0 && lastGranteeIdx < len(cc.PCfg.FeeGrants.ManagedGrantees) {
		txSignerKey = cc.PCfg.FeeGrants.ManagedGrantees[lastGranteeIdx]
		cc.PCfg.FeeGrants.GranteeLastSignerIndex++

		// Restart the round-robin if we reached the end of the list of grantees
		if cc.PCfg.FeeGrants.GranteeLastSignerIndex >= len(cc.PCfg.FeeGrants.ManagedGrantees) {
			cc.PCfg.FeeGrants.GranteeLastSignerIndex = 0
		}
	}

	return txSignerKey, feeGranterKeyOrAddr
}
