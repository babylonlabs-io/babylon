package rollup

type CommitPublicRandomnessMsg struct {
	CommitPublicRandomness CommitPublicRandomnessMsgParams `json:"commit_public_randomness"`
}

type CommitPublicRandomnessMsgParams struct {
	FpPubkeyHex string `json:"fp_pubkey_hex"`
	StartHeight uint64 `json:"start_height"`
	NumPubRand  uint64 `json:"num_pub_rand"`
	Commitment  []byte `json:"commitment"`
	Signature   []byte `json:"signature"`
}

// TODO: need to update based on contract implementation
type CommitPublicRandomnessResponse struct {
	Result bool `json:"result"`
}

type SubmitFinalitySignatureMsg struct {
	SubmitFinalitySignature SubmitFinalitySignatureMsgParams `json:"submit_finality_signature"`
}

type SubmitFinalitySignatureMsgParams struct {
	FpPubkeyHex string `json:"fp_pubkey_hex"`
	Height      uint64 `json:"height"`
	PubRand     []byte `json:"pub_rand"`
	Proof       Proof  `json:"proof"`
	BlockHash   []byte `json:"block_hash"`
	Signature   []byte `json:"signature"`
}

// TODO: need to update based on contract implementation
type SubmitFinalitySignatureResponse struct {
	Result bool `json:"result"`
}

type QueryMsg struct {
	Config             *Config        `json:"config,omitempty"`
	FirstPubRandCommit *PubRandCommit `json:"first_pub_rand_commit,omitempty"`
	LastPubRandCommit  *PubRandCommit `json:"last_pub_rand_commit,omitempty"`
	// BlockVoters is used to query the voters for a specific block
	BlockVoters *BlockVoters `json:"block_voters,omitempty"`
}

type Config struct{}

type PubRandCommit struct {
	BtcPkHex string `json:"btc_pk_hex"`
}

// FIXME: Remove this ancillary struct.
// Only required because the e2e tests are using a zero index, which is removed by the `json:"omitempty"` annotation in
// the original cmtcrypto Proof
type Proof struct {
	Total    uint64   `json:"total"`
	Index    uint64   `json:"index"`
	LeafHash []byte   `json:"leaf_hash"`
	Aunts    [][]byte `json:"aunts"`
}

type PubRandCommitResponse struct {
	StartHeight uint64 `json:"start_height"`
	NumPubRand  uint64 `json:"num_pub_rand"`
	Commitment  []byte `json:"commitment"`
}

type BlockVoters struct {
	Height uint64 `json:"height"`
	// The block app hash is expected to be in hex format, without the 0x prefix
	HashHex string `json:"hash_hex"`
}

// List of finality provider public keys who voted for the block
type BlockVotersResponse []string
