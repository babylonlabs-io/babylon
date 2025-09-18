package keepers

import (
	"fmt"
	"maps"

	"cosmossdk.io/core/address"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/cosmos-sdk/codec"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	bankprecompile "github.com/cosmos/evm/precompiles/bank"
	"github.com/cosmos/evm/precompiles/bech32"
	govprecompile "github.com/cosmos/evm/precompiles/gov"
	"github.com/cosmos/evm/precompiles/p256"
	slashingprecompile "github.com/cosmos/evm/precompiles/slashing"
	erc20Keeper "github.com/cosmos/evm/x/erc20/keeper"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	epochingprecompile "github.com/babylonlabs-io/babylon/v4/precompiles/epoching"
	checkpointingkeeper "github.com/babylonlabs-io/babylon/v4/x/checkpointing/keeper"
	epochingkeeper "github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
)

const bech32PrecompileBaseGas = 6_000

// BabylonAvailableStaticPrecompiles defines the full list of all available EVM extension addresses on Babylon.
//
// NOTE: To be explicit, this list does not include the dynamically registered EVM extensions
// like the ERC-20 extensions.
var BabylonAvailableStaticPrecompiles = []string{
	evmtypes.P256PrecompileAddress,
	evmtypes.Bech32PrecompileAddress,
	evmtypes.BankPrecompileAddress,
	evmtypes.GovPrecompileAddress,
	evmtypes.SlashingPrecompileAddress,
	epochingprecompile.EpochingPrecompileAddress,
}

type PrecompileOptions struct {
	// Codec is the codec used to encode and decode messages.
	AddressCodec address.Codec
	// ValidatorAddressCodec is the codec used to encode and decode validator addresses.
	ValidatorAddressCodec address.Codec
	// ConsensusAddressCodec is the codec used to encode and decode consensus addresses.
	ConsensusAddressCodec address.Codec
}

var CodecOptions = PrecompileOptions{
	AddressCodec:          authcodec.NewBech32Codec(appparams.Bech32PrefixAccAddr),
	ValidatorAddressCodec: authcodec.NewBech32Codec(appparams.Bech32PrefixValAddr),
	ConsensusAddressCodec: authcodec.NewBech32Codec(appparams.Bech32PrefixConsAddr),
}

// NewAvailableStaticPrecompiles adds the static precompiles to the EVM
// TODO: Add custom staking wrapper precompile here, IBC precompile and distribution precompile
func NewAvailableStaticPrecompiles(
	cdc codec.Codec,
	bankKeeper precisebankkeeper.Keeper,
	erc20Keeper erc20Keeper.Keeper,
	govKeeper govkeeper.Keeper,
	slashingKeeper slashingkeeper.Keeper,
	stakingKeeper *stakingkeeper.Keeper,
	epochingKeeper epochingkeeper.Keeper,
	checkpointingKeeper checkpointingkeeper.Keeper,
) map[common.Address]vm.PrecompiledContract {
	// TODO: We can add more custom precompiles here for Babylon Modules
	// Clone the mapping from the latest EVM fork.
	precompiles := maps.Clone(vm.PrecompiledContractsPrague)

	// secp256r1 precompile as per EIP-7212
	p256Precompile := &p256.Precompile{}

	bech32Precompile, err := bech32.NewPrecompile(bech32PrecompileBaseGas)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate bech32 precompile: %w", err))
	}

	bankPrecompile, err := bankprecompile.NewPrecompile(bankKeeper, erc20Keeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate bank precompile: %w", err))
	}

	govPrecompile, err := govprecompile.NewPrecompile(govKeeper, cdc, CodecOptions.AddressCodec)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate gov precompile: %w", err))
	}

	slashingPrecompile, err := slashingprecompile.NewPrecompile(slashingKeeper, CodecOptions.ValidatorAddressCodec, CodecOptions.ConsensusAddressCodec)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate slashing precompile: %w", err))
	}

	epochingMsgServer := epochingkeeper.NewMsgServerImpl(epochingKeeper)
	epochingQuerier := epochingkeeper.Querier{Keeper: epochingKeeper}
	checkpointingMsgServer := checkpointingkeeper.NewMsgServerImpl(checkpointingKeeper)
	stakingQuerier := stakingkeeper.Querier{Keeper: stakingKeeper}

	epochingPrecompile, err := epochingprecompile.NewPrecompile(epochingKeeper, epochingMsgServer, epochingQuerier, checkpointingMsgServer, *stakingKeeper, stakingQuerier, CodecOptions.AddressCodec, CodecOptions.ValidatorAddressCodec)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate epoching precompile: %w", err))
	}

	// Stateless precompiles
	precompiles[bech32Precompile.Address()] = bech32Precompile
	precompiles[p256Precompile.Address()] = p256Precompile

	// Stateful precompiles
	precompiles[bankPrecompile.Address()] = bankPrecompile
	precompiles[govPrecompile.Address()] = govPrecompile
	precompiles[slashingPrecompile.Address()] = slashingPrecompile
	precompiles[epochingPrecompile.Address()] = epochingPrecompile

	return precompiles
}
