package cosmwasm

type ConsumerFpsResponse struct {
	Fps []SingleConsumerFpResponse `json:"fps"`
}

// SingleConsumerFpResponse represents the finality provider data returned by the contract query.
// For more details, refer to the following links:
// https://github.com/babylonchain/babylon-contract/blob/v0.5.3/packages/apis/src/btc_staking_api.rs
// https://github.com/babylonchain/babylon-contract/blob/v0.5.3/contracts/btc-staking/src/msg.rs
// https://github.com/babylonchain/babylon-contract/blob/v0.5.3/contracts/btc-staking/schema/btc-staking.json
type SingleConsumerFpResponse struct {
	BtcPkHex         string `json:"btc_pk_hex"`
	SlashedHeight    uint64 `json:"slashed_height"`
	SlashedBtcHeight uint32 `json:"slashed_btc_height"`
	ConsumerId       string `json:"consumer_id"`
}

type ConsumerDelegationsResponse struct {
	Delegations []SingleConsumerDelegationResponse `json:"delegations"`
}

type SingleConsumerDelegationResponse struct {
	BtcPkHex             string                      `json:"btc_pk_hex"`
	FpBtcPkList          []string                    `json:"fp_btc_pk_list"`
	StartHeight          uint32                      `json:"start_height"`
	EndHeight            uint32                      `json:"end_height"`
	TotalSat             uint64                      `json:"total_sat"`
	StakingTx            []byte                      `json:"staking_tx"`
	SlashingTx           []byte                      `json:"slashing_tx"`
	DelegatorSlashingSig []byte                      `json:"delegator_slashing_sig"`
	CovenantSigs         []CovenantAdaptorSignatures `json:"covenant_sigs"`
	StakingOutputIdx     uint32                      `json:"staking_output_idx"`
	UnbondingTime        uint32                      `json:"unbonding_time"`
	UndelegationInfo     *BtcUndelegationInfo        `json:"undelegation_info"`
	ParamsVersion        uint32                      `json:"params_version"`
}

type ConsumerFpInfoResponse struct {
	BtcPkHex string `json:"btc_pk_hex"`
	Power    uint64 `json:"power"`
}

type ConsumerFpsByPowerResponse struct {
	Fps []ConsumerFpInfoResponse `json:"fps"`
}

type FinalitySignatureResponse struct {
	Signature []byte `json:"signature"`
}

type BlocksResponse struct {
	Blocks []IndexedBlock `json:"blocks"`
}

type IndexedBlock struct {
	Height    uint64 `json:"height"`
	AppHash   []byte `json:"app_hash"`
	Finalized bool   `json:"finalized"`
}

type NewFinalityProvider struct {
	Description *FinalityProviderDescription `json:"description,omitempty"`
	Commission  string                       `json:"commission"`
	Addr        string                       `json:"addr"`
	BTCPKHex    string                       `json:"btc_pk_hex"`
	Pop         *ProofOfPossessionBtc        `json:"pop,omitempty"`
	ConsumerID  string                       `json:"consumer_id"`
}

type FinalityProviderDescription struct {
	Moniker         string `json:"moniker"`
	Identity        string `json:"identity"`
	Website         string `json:"website"`
	SecurityContact string `json:"security_contact"`
	Details         string `json:"details"`
}

type ProofOfPossessionBtc struct {
	BTCSigType int32  `json:"btc_sig_type"`
	BTCSig     []byte `json:"btc_sig"`
}

type CovenantAdaptorSignatures struct {
	CovPK       []byte   `json:"cov_pk"`
	AdaptorSigs [][]byte `json:"adaptor_sigs"`
}

type SignatureInfo struct {
	PK  []byte `json:"pk"`
	Sig []byte `json:"sig"`
}

type BtcUndelegationInfo struct {
	UnbondingTx              []byte                       `json:"unbonding_tx"`
	SlashingTx               []byte                       `json:"slashing_tx"`
	DelegatorSlashingSig     []byte                       `json:"delegator_slashing_sig"`
	CovenantSlashingSigs     []*CovenantAdaptorSignatures `json:"covenant_slashing_sigs"`
	CovenantUnbondingSigList []*SignatureInfo             `json:"covenant_unbonding_sig_list"`
	DelegatorUnbondingInfo   *DelegatorUnbondingInfo      `json:"delegator_unbonding_info"`
}

type DelegatorUnbondingInfo struct {
	SpendStakeTx []byte `json:"spend_stake_tx"`
}

type ActiveBtcDelegation struct {
	StakerAddr           string                      `json:"staker_addr"`
	BTCPkHex             string                      `json:"btc_pk_hex"`
	FpBtcPkList          []string                    `json:"fp_btc_pk_list"`
	StartHeight          uint32                      `json:"start_height"`
	EndHeight            uint32                      `json:"end_height"`
	TotalSat             uint64                      `json:"total_sat"`
	StakingTx            []byte                      `json:"staking_tx"`
	SlashingTx           []byte                      `json:"slashing_tx"`
	DelegatorSlashingSig []byte                      `json:"delegator_slashing_sig"`
	CovenantSigs         []CovenantAdaptorSignatures `json:"covenant_sigs"`
	StakingOutputIdx     uint32                      `json:"staking_output_idx"`
	UnbondingTime        uint32                      `json:"unbonding_time"`
	UndelegationInfo     BtcUndelegationInfo         `json:"undelegation_info"`
	ParamsVersion        uint32                      `json:"params_version"`
}

type SlashedBtcDelegation struct {
	// Define fields as needed
}

type UnbondedBtcDelegation struct {
	// Define fields as needed
}

type BtcStaking struct {
	NewFP       []NewFinalityProvider   `json:"new_fp"`
	ActiveDel   []ActiveBtcDelegation   `json:"active_del"`
	SlashedDel  []SlashedBtcDelegation  `json:"slashed_del"`
	UnbondedDel []UnbondedBtcDelegation `json:"unbonded_del"`
}

type CommitPublicRandomness struct {
	FPPubKeyHex string `json:"fp_pubkey_hex"`
	StartHeight uint64 `json:"start_height"`
	NumPubRand  uint64 `json:"num_pub_rand"`
	Commitment  []byte `json:"commitment"`
	Signature   []byte `json:"signature"`
}

type Proof struct {
	Total    int64    `json:"total"`
	Index    int64    `json:"index"`
	LeafHash []byte   `json:"leaf_hash"`
	Aunts    [][]byte `json:"aunts"`
}

type SubmitFinalitySignature struct {
	FpPubkeyHex string `json:"fp_pubkey_hex"`
	Height      uint64 `json:"height"`
	PubRand     []byte `json:"pub_rand"`
	Proof       Proof  `json:"proof"` // nested struct
	BlockHash   []byte `json:"block_hash"`
	Signature   []byte `json:"signature"`
}

type ExecMsg struct {
	SubmitFinalitySignature *SubmitFinalitySignature `json:"submit_finality_signature,omitempty"`
	BtcStaking              *BtcStaking              `json:"btc_staking,omitempty"`
	CommitPublicRandomness  *CommitPublicRandomness  `json:"commit_public_randomness,omitempty"`
}

type FinalityProviderInfo struct {
	BtcPkHex string `json:"btc_pk_hex"`
	Height   uint64 `json:"height,omitempty"`
}

type QueryMsgFinalityProviderInfo struct {
	FinalityProviderInfo FinalityProviderInfo `json:"finality_provider_info"`
}

type BlockQuery struct {
	Height uint64 `json:"height"`
}

type QueryMsgBlock struct {
	Block BlockQuery `json:"block"`
}

type QueryMsgBlocks struct {
	Blocks BlocksQuery `json:"blocks"`
}

type BlocksQuery struct {
	StartAfter *uint64 `json:"start_after,omitempty"`
	Limit      *uint64 `json:"limit,omitempty"`
	Finalized  *bool   `json:"finalised,omitempty"` //TODO: finalised or finalized, typo in smart contract
	Reverse    *bool   `json:"reverse,omitempty"`
}

type QueryMsgActivatedHeight struct {
	ActivatedHeight struct{} `json:"activated_height"`
}

type QueryMsgFinalitySignature struct {
	FinalitySignature FinalitySignatureQuery `json:"finality_signature"`
}

type FinalitySignatureQuery struct {
	BtcPkHex string `json:"btc_pk_hex"`
	Height   uint64 `json:"height"`
}

type QueryMsgFinalityProviders struct {
	FinalityProviders struct{} `json:"finality_providers"`
}

type QueryMsgFinalityProvider struct {
	FinalityProvider FinalityProviderQuery `json:"finality_provider"`
}

type FinalityProviderQuery struct {
	BtcPkHex string `json:"btc_pk_hex"`
}

type QueryMsgDelegations struct {
	Delegations struct{} `json:"delegations"`
}

type QueryMsgFinalityProvidersByPower struct {
	FinalityProvidersByPower struct{} `json:"finality_providers_by_power"`
}

type QueryMsgLastPubRandCommit struct {
	LastPubRandCommit LastPubRandCommitQuery `json:"last_pub_rand_commit"`
}

type LastPubRandCommitQuery struct {
	BtcPkHex string `json:"btc_pk_hex"`
}
