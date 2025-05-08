package containers

// ImageConfig contains all images and their respective tags
// needed for running e2e tests.
type ImageConfig struct {
	// IBC relayer for cosmos-SDK
	RelayerRepository string
	RelayerTag        string

	CurrentRepository string
	CurrentTag        string
}

//nolint:deadcode
const (
	// Images that do not have specified tag, latest will be used by default.
	// name of babylon image produced by running `make build-docker`
	BabylonContainerName = "babylonlabs-io/babylond"
	// name of babylon image before the upgrade
	BabylonContainerNameBeforeUpgrade = "babylonlabs/babylond"
	BabylonContainerTagBeforeUpgrade  = "v1.0.1"

	// name of the image produced by running `make e2e-init-chain` in contrib/images
	InitChainContainerE2E = "babylonlabs-io/babylond-e2e-init-chain"

	hermesRelayerRepository = "informalsystems/hermes"
	hermesRelayerTag        = "v1.8.2"
	// Built using the `build-cosmos-relayer-docker` target on an Intel (amd64) machine and pushed to ECR
	cosmosRelayerRepository = "public.ecr.aws/t9e9i3h0/cosmos-relayer"
	// TODO: Replace with version tag once we have a working version
	cosmosRelayerTag = "main"
)

// NewImageConfig returns ImageConfig needed for running e2e test.
// If isUpgrade is true, returns images for running the upgrade
// If isFork is true, utilizes provided fork height to initiate fork logic
func NewImageConfig(isCosmosRelayer, isUpgrade bool) (ic ImageConfig) {
	ic = ImageConfig{
		CurrentRepository: BabylonContainerName,
		CurrentTag:        "latest",
	}

	if isUpgrade {
		// starts at the older version and later upgrades it to current branch... BabylonContainerName
		ic.CurrentRepository = BabylonContainerNameBeforeUpgrade
		ic.CurrentTag = BabylonContainerTagBeforeUpgrade
	}

	if isCosmosRelayer {
		ic.RelayerRepository = cosmosRelayerRepository
		ic.RelayerTag = cosmosRelayerTag
		return ic
	} else {
		ic.RelayerRepository = hermesRelayerRepository
		ic.RelayerTag = hermesRelayerTag
		return ic
	}
}
