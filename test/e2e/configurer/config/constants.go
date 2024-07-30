package config

const (
	// PropDepositBlocks estimated number of blocks it takes to deposit for a proposal
	PropDepositBlocks float32 = 10
	// PropVoteBlocks number of blocks it takes to vote for a single validator to vote for a proposal
	PropVoteBlocks float32 = 1.2
	// PropBufferBlocks number of blocks used as a calculation buffer
	PropBufferBlocks float32 = 6

	// Upgrades
	// ForkHeightPreUpgradeOffset how many blocks we allow for fork to run pre upgrade state creation
	ForkHeightPreUpgradeOffset int64 = 60
	// MaxRetries for json unmarshalling init chain
	MaxRetries = 60
	// PropSubmitBlocks estimated number of blocks it takes to submit for a proposal
	PropSubmitBlocks float32 = 1
	// VanillaUpgradeFilePath upgrade vanilla testing
	VanillaUpgradeFilePath = "/upgrades/vanilla.json"
)
