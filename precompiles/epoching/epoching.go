package epoching

import (
	"embed"

	"cosmossdk.io/core/address"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
)

var _ vm.PrecompiledContract = &Precompile{}

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

// Precompile defines the precompiled contract for epoching.
type Precompile struct {
	cmn.Precompile
	epochingKeeper         keeper.Keeper
	epochingMsgServer      epochingtypes.MsgServer
	epochingQuerier        epochingtypes.QueryServer
	checkpointingMsgServer checkpointingtypes.MsgServer
	stakingKeeper          stakingkeeper.Keeper
	stakingQuerier         stakingtypes.QueryServer
	addrCdc, valCdc        address.Codec
}

// LoadABI loads the epoching ABI from the embedded abi.json file
// for the epoching precompile.
func LoadABI() (abi.ABI, error) {
	return cmn.LoadABI(f, "abi.json")
}

// NewPrecompile creates a new epoching Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	epochingKeeper keeper.Keeper,
	epochingMsgServer epochingtypes.MsgServer,
	epochingQuerier epochingtypes.QueryServer,
	checkpointingMsgServer checkpointingtypes.MsgServer,
	stakingKeeper stakingkeeper.Keeper,
	stakingQuerier stakingtypes.QueryServer,
	addrCdc, valCdc address.Codec,
) (*Precompile, error) {
	abi, err := LoadABI()
	if err != nil {
		return nil, err
	}

	p := &Precompile{
		Precompile: cmn.Precompile{
			ABI:                  abi,
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
		},
		epochingKeeper:         epochingKeeper,
		epochingMsgServer:      epochingMsgServer,
		epochingQuerier:        epochingQuerier,
		checkpointingMsgServer: checkpointingMsgServer,
		stakingKeeper:          stakingKeeper,
		stakingQuerier:         stakingQuerier,
		addrCdc:                addrCdc,
		valCdc:                 valCdc,
	}
	// SetAddress defines the address of the epoching precompiled contract.
	p.SetAddress(common.HexToAddress(EpochingPrecompileAddress))

	return p, nil
}

func (p Precompile) RequiredGas(input []byte) uint64 {
	// NOTE: This check avoid panicking when trying to decode the method ID
	if len(input) < 4 {
		return 0
	}

	methodID := input[:4]

	method, err := p.MethodById(methodID)
	if err != nil {
		// This should never happen since this method is going to fail during Run
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(method))
}

// Run executes the precompiled contract epoching methods defined in the ABI.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	bz, err = p.run(evm, contract, readOnly)
	if err != nil {
		return cmn.ReturnRevertError(evm, err)
	}
	return bz, nil
}

func (p Precompile) run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	ctx, stateDB, method, initialGas, args, err := p.RunSetup(evm, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	// Start the balance change handler before executing the precompile.
	p.GetBalanceHandler().BeforeBalanceChange(ctx)

	// This handles any out of gas errors that may occur during the execution of a precompile tx or query.
	// It avoids panics and returns the out of gas error so the EVM can continue gracefully.
	defer cmn.HandleGasError(ctx, contract, initialGas, &err)()

	switch method.Name {
	// Epoching transactions
	case WrappedCreateValidatorMethod:
		bz, err = p.WrappedCreateValidator(ctx, contract, stateDB, method, args)
	case WrappedEditValidatorMethod:
		bz, err = p.WrappedEditValidator(ctx, contract, stateDB, method, args)
	case WrappedDelegateMethod:
		bz, err = p.WrappedDelegate(ctx, contract, stateDB, method, args)
	case WrappedDelegateBech32Method:
		bz, err = p.WrappedDelegateBech32(ctx, contract, stateDB, method, args)
	case WrappedUndelegateMethod:
		bz, err = p.WrappedUndelegate(ctx, contract, stateDB, method, args)
	case WrappedUndelegateBech32Method:
		bz, err = p.WrappedUndelegateBech32(ctx, contract, stateDB, method, args)
	case WrappedRedelegateMethod:
		bz, err = p.WrappedRedelegate(ctx, contract, stateDB, method, args)
	case WrappedRedelegateBech32Method:
		bz, err = p.WrappedRedelegateBech32(ctx, contract, stateDB, method, args)
	case WrappedCancelUnbondingDelegationMethod:
		bz, err = p.WrappedCancelUnbondingDelegation(ctx, contract, stateDB, method, args)
	case WrappedCancelUnbondingDelegationBech32Method:
		bz, err = p.WrappedCancelUnbondingDelegationBech32(ctx, contract, stateDB, method, args)
	// Epoching queries
	case EpochInfoMethod:
		bz, err = p.EpochInfo(ctx, contract, method, args)
	case CurrentEpochMethod:
		bz, err = p.CurrentEpoch(ctx, contract, method, args)
	case EpochMsgsMethod:
		bz, err = p.EpochMsgs(ctx, contract, method, args)
	case ValidatorLifecycleMethod:
		bz, err = p.ValidatorLifecycle(ctx, contract, method, args)
	case DelegationLifecycleMethod:
		bz, err = p.DelegationLifecycle(ctx, contract, method, args)
	case DelegationLifecycleBech32Method:
		bz, err = p.DelegationLifecycleBech32(ctx, contract, method, args)
	case EpochValSetMethod:
		bz, err = p.EpochValSet(ctx, contract, method, args)
	// Staking queries
	case DelegationMethod:
		bz, err = p.Delegation(ctx, contract, method, args)
	case DelegationBech32Method:
		bz, err = p.DelegationBech32(ctx, contract, method, args)
	case UnbondingDelegationMethod:
		bz, err = p.UnbondingDelegation(ctx, contract, method, args)
	case UnbondingDelegationBech32Method:
		bz, err = p.UnbondingDelegationBech32(ctx, contract, method, args)
	case ValidatorMethod:
		bz, err = p.Validator(ctx, method, contract, args)
	case ValidatorBech32Method:
		bz, err = p.ValidatorBech32(ctx, method, contract, args)
	case ValidatorsMethod:
		bz, err = p.Validators(ctx, method, contract, args)
	case ValidatorsBech32Method:
		bz, err = p.ValidatorsBech32(ctx, method, contract, args)
	case RedelegationMethod:
		bz, err = p.Redelegation(ctx, method, contract, args)
	case RedelegationBech32Method:
		bz, err = p.RedelegationBech32(ctx, method, contract, args)
	case RedelegationsMethod:
		bz, err = p.Redelegations(ctx, method, contract, args)
	case RedelegationsBech32Method:
		bz, err = p.RedelegationsBech32(ctx, method, contract, args)
	}

	if err != nil {
		return nil, err
	}

	cost := ctx.GasMeter().GasConsumed() - initialGas

	if !contract.UseGas(cost, nil, tracing.GasChangeCallPrecompiledContract) {
		return nil, vm.ErrOutOfGas
	}

	// Process the native balance changes after the method execution.
	if err = p.GetBalanceHandler().AfterBalanceChange(ctx, stateDB); err != nil {
		return nil, err
	}

	return bz, nil
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
//
// Available epoching transactions are:
//   - WrappedCreateValidator
//   - WrappedEditValidator
//   - WrappedDelegate
//   - WrappedUndelegate
//   - WrappedRedelegate
//   - WrappedCancelUnbondingDelegation
//   - WrappedDelegateBech32
//   - WrappedUndelegateBech32
//   - WrappedRedelegateBech32
//   - WrappedCancelUnbondingDelegationBech32
func (p Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case WrappedCreateValidatorMethod,
		WrappedEditValidatorMethod,
		WrappedDelegateMethod,
		WrappedUndelegateMethod,
		WrappedRedelegateMethod,
		WrappedCancelUnbondingDelegationMethod,
		WrappedDelegateBech32Method,
		WrappedUndelegateBech32Method,
		WrappedRedelegateBech32Method,
		WrappedCancelUnbondingDelegationBech32Method:
		return true
	default:
		return false
	}
}

func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("babylon extension", "epoching")
}
