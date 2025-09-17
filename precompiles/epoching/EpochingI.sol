// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

import "../common/Types.sol";

/// @dev The EpochingI contract's address.
address constant EPOCHING_PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000001000;

/// @dev The EpochingI contract's instance.
EpochingI constant EPOCHING_CONTRACT = EpochingI(EPOCHING_PRECOMPILE_ADDRESS);

/// @dev Define all the available epoching methods.
string constant MSG_WRAPPED_CREATE_VALIDATOR = "/babylon.checkpointing.v1.MsgWrappedCreateValidator";
string constant MSG_WRAPPED_EDIT_VALIDATOR = "/babylon.epoching.v1.MsgWrappedEditValidator";
string constant MSG_WRAPPED_DELEGATE = "/babylon.epoching.v1.MsgWrappedDelegate";
string constant MSG_WRAPPED_UNDELEGATE = "/babylon.epoching.v1.MsgWrappedUndelegate";
string constant MSG_WRAPPED_REDELEGATE = "/babylon.epoching.v1.MsgWrappedBeginRedelegate";
string constant MSG_WRAPPED_CANCEL_UNDELEGATION = "/babylon.epoching.v1.MsgWrappedCancelUnbondingDelegation";

/// @dev Constant used in flags to indicate that commission rate field should not be updated
int256 constant DO_NOT_MODIFY_COMMISSION_RATE = -1;

/// @dev Constant used in flags to indicate that min self delegation field should not be updated
int256 constant DO_NOT_MODIFY_MIN_SELF_DELEGATION = -1;

/// @dev Defines the blskey to be used for creating validator
    struct BlsKey {
        bytes pubKey;
        bytes ed25519Sig;
        bytes blsSig;
    }

/// @dev Defines the initial description to be used for creating
/// a validator.
    struct Description {
        string moniker;
        string identity;
        string website;
        string securityContact;
        string details;
    }

/// @dev Defines the initial commission rates to be used for creating
/// a validator.
    struct CommissionRates {
        uint256 rate;
        uint256 maxRate;
        uint256 maxChangeRate;
    }

/// @dev Defines commission parameters for a given validator.
    struct Commission {
        CommissionRates commissionRates;
        uint256 updateTime;
    }

/// @dev Represents a validator in the epoching module with bech32 addresses.
    struct ValidatorBech32 {
        string operatorAddress;
        string consensusPubkey;
        bool jailed;
        BondStatus status;
        uint256 tokens;
        uint256 delegatorShares; // TODO: decimal
        string description;
        int64 unbondingHeight;
        int64 unbondingTime;
        uint256 commission;
        uint256 minSelfDelegation;
    }

/// @dev Represents a validator in the epoching module with hex addresses.
    struct Validator {
        address operatorAddress;
        string consensusPubkey;
        bool jailed;
        BondStatus status;
        uint256 tokens;
        uint256 delegatorShares; // TODO: decimal
        string description;
        int64 unbondingHeight;
        int64 unbondingTime;
        uint256 commission;
        uint256 minSelfDelegation;
    }

/// @dev Represents the output of a Redelegations query with bech32 addresses.
    struct RedelegationResponseBech32 {
        RedelegationBech32 redelegation;
        RedelegationEntryResponse[] entries;
    }

/// @dev Represents the output of a Redelegations query with hex addresses.
    struct RedelegationResponse {
        Redelegation redelegation;
        RedelegationEntryResponse[] entries;
    }

/// @dev Represents a redelegation between a delegator and a validator with bech32 addresses.
    struct RedelegationBech32 {
        string delegatorAddress;
        string validatorSrcAddress;
        string validatorDstAddress;
        RedelegationEntry[] entries;
    }

/// @dev Represents a redelegation between a delegator and a validator with hex addresses.
    struct Redelegation {
        address delegatorAddress;
        address validatorSrcAddress;
        address validatorDstAddress;
        RedelegationEntry[] entries;
    }

/// @dev Represents a RedelegationEntryResponse for the Redelegations query.
    struct RedelegationEntryResponse {
        RedelegationEntry redelegationEntry;
        uint256 balance;
    }

/// @dev Represents a single Redelegation entry.
    struct RedelegationEntry {
        int64 creationHeight;
        int64 completionTime;
        uint256 initialBalance;
        uint256 sharesDst; // TODO: decimal
    }

/// @dev Represents the output of the Redelegation query with bech32 addresses.
    struct RedelegationOutputBech32 {
        string delegatorAddress;
        string validatorSrcAddress;
        string validatorDstAddress;
        RedelegationEntry[] entries;
    }

/// @dev Represents the output of the Redelegation query with hex addresses.
    struct RedelegationOutput {
        address delegatorAddress;
        address validatorSrcAddress;
        address validatorDstAddress;
        RedelegationEntry[] entries;
    }

/// @dev Represents a single entry of an unbonding delegation.
    struct UnbondingDelegationEntry {
        int64 creationHeight;
        int64 completionTime;
        uint256 initialBalance;
        uint256 balance;
        uint64 unbondingId;
        int64 unbondingOnHoldRefCount;
    }

/// @dev Represents the output of the UnbondingDelegation query.
    struct UnbondingDelegationOutputBech32 {
        string delegatorAddress;
        string validatorAddress;
        UnbondingDelegationEntry[] entries;
    }

/// @dev Represents the output of the UnbondingDelegation query.
    struct UnbondingDelegationOutput {
        address delegatorAddress;
        address validatorAddress;
        UnbondingDelegationEntry[] entries;
    }

/// @dev Represents epoch information response
    struct EpochResponse {
        uint64 epochNumber;
        uint64 currentEpochInterval;
        uint64 firstBlockHeight;
        int64 lastBlockTime;
        string sealerAppHashHex;
        string sealerBlockHash;
    }

/// @dev Represents current epoch information response
    struct CurrentEpochResponse {
        uint64 currentEpoch;
        uint64 epochBoundary;
    }

/// @dev Represents a queued message in an epoch
    struct QueuedMessageResponse {
        string txId;
        string msgId;
        uint64 blockHeight;
        int64 blockTime;
        string msg;
        string msgType;
    }

/// @dev Represents a list of queued messages for an epoch
    struct QueuedMessageList {
        uint64 epochNumber;
        QueuedMessageResponse[] msgs;
    }

/// @dev Represents a validator lifecycle update event
    struct ValidatorUpdateResponse {
        string stateDesc;
        uint64 blockHeight;
        int64 blockTime;
    }

/// @dev Represents a delegation state change event with bech32 addresses
    struct DelegationStateUpdateBech32 {
        BondState state;
        string valAddr;
        Coin amount;
        uint64 blockHeight;
        int64 blockTime;
    }

/// @dev Represents a delegation state change event with hex addresses
    struct DelegationStateUpdate {
        BondState state;
        address valAddr;
        Coin amount;
        uint64 blockHeight;
        int64 blockTime;
    }

/// @dev Represents the complete lifecycle of a delegation with bech32 addresses
    struct DelegationLifecycleBech32 {
        string delAddr;
        DelegationStateUpdateBech32[] delLife;
    }

/// @dev Represents the complete lifecycle of a delegation with hex addresses
    struct DelegationLifecycle {
        address delAddr;
        DelegationStateUpdate[] delLife;
    }

/// @dev Represents a simple validator with address and voting power
    struct SimpleValidator {
        address addr;
        int64 power;
    }

/// @dev The status of the validator.
    enum BondStatus {
        Unspecified,
        Unbonded,
        Unbonding,
        Bonded
    }

/// @dev The status of the delegator
    enum BondState {
        CREATED,
        BONDED,
        UNBONDING,
        UNBONDED,
        REMOVED
    }

/// @author Babylon Team
/// @title Epoching Precompiled Contract
/// @dev The interface through which solidity contracts will interact with Epoching.
/// We follow this same interface including four-byte function selectors, in the precompile that
/// wraps the pallet.
/// @custom:address 0x0000000000000000000000000000000000001000
interface EpochingI {
    /// @dev Defines a method for creating a new validator.
    /// @param blsKey The validator's blsKey
    /// @param description The initial description
    /// @param commissionRates The initial commissionRates
    /// @param minSelfDelegation The validator's self declared minimum self delegation
    /// @param validatorAddress The validator address
    /// @param pubkey The consensus public key of the validator
    /// @param value The amount of the coin to be self delegated to the validator
    /// @return success Whether or not the create validator was successful
    function wrappedCreateValidator(
        BlsKey calldata blsKey,
        Description calldata description,
        CommissionRates calldata commissionRates,
        uint256 minSelfDelegation,
        address validatorAddress,
        string memory pubkey,
        uint256 value
    ) external returns (bool success);

    /// @dev Defines a method for edit a validator.
    /// @param description Description parameter to be updated. Use the string "[do-not-modify]"
    /// as the value of fields that should not be updated.
    /// @param commissionRate CommissionRate parameter to be updated.
    /// Use commissionRate = -1 to keep the current value and not update it.
    /// @param minSelfDelegation MinSelfDelegation parameter to be updated.
    /// Use minSelfDelegation = -1 to keep the current value and not update it.
    /// @return success Whether or not edit validator was successful.
    function wrappedEditValidator(
        Description calldata description,
        address validatorAddress,
        int256 commissionRate,
        int256 minSelfDelegation
    ) external returns (bool success);

    /// @dev Defines a method for performing a delegation of coins from a delegator to a validator.
    /// @param delegatorAddress The address of the delegator
    /// @param validatorAddress The address of the validator
    /// @param amount The amount of the bond denomination to be delegated to the validator.
    /// This amount should use the bond denomination precision stored in the bank metadata.
    /// @return success Whether or not the delegate was successful
    function wrappedDelegateBech32(
        address delegatorAddress,
        string memory validatorAddress,
        uint256 amount
    ) external returns (bool success);

    /// @dev Defines a method for performing a delegation of coins from a delegator to a validator.
    /// @param delegatorAddress The address of the delegator
    /// @param validatorAddress The address of the validator
    /// @param amount The amount of the bond denomination to be delegated to the validator.
    /// This amount should use the bond denomination precision stored in the bank metadata.
    /// @return success Whether or not the delegate was successful
    function wrappedDelegate(
        address delegatorAddress,
        address validatorAddress,
        uint256 amount
    ) external returns (bool success);

    /// @dev Defines a method for performing an undelegation from a delegate and a validator.
    /// @param delegatorAddress The address of the delegator
    /// @param validatorAddress The address of the validator
    /// @param amount The amount of the bond denomination to be undelegated from the validator.
    /// This amount should use the bond denomination precision stored in the bank metadata.
    /// @return success Whether or not the redelegate was successfully enqueued
    function wrappedUndelegateBech32(
        address delegatorAddress,
        string memory validatorAddress,
        uint256 amount
    ) external returns (bool success);

    /// @dev Defines a method for performing an undelegation from a delegate and a validator.
    /// @param delegatorAddress The address of the delegator
    /// @param validatorAddress The address of the validator
    /// @param amount The amount of the bond denomination to be undelegated from the validator.
    /// This amount should use the bond denomination precision stored in the bank metadata.
    /// @return success Whether or not the redelegate was successfully enqueued
    function wrappedUndelegate(
        address delegatorAddress,
        address validatorAddress,
        uint256 amount
    ) external returns (bool success);

    /// @dev Defines a method for performing a redelegation
    /// of coins from a delegator and source validator to a destination validator.
    /// @param delegatorAddress The address of the delegator
    /// @param validatorSrcAddress The validator from which the redelegation is initiated
    /// @param validatorDstAddress The validator to which the redelegation is destined
    /// @param amount The amount of the bond denomination to be redelegated to the validator
    /// This amount should use the bond denomination precision stored in the bank metadata.
    /// @return success Whether or not the redelegate was successfully enqueued
    function wrappedRedelegateBech32(
        address delegatorAddress,
        string memory validatorSrcAddress,
        string memory validatorDstAddress,
        uint256 amount
    ) external returns (bool success);

    /// @dev Defines a method for performing a redelegation
    /// of coins from a delegator and source validator to a destination validator.
    /// @param delegatorAddress The address of the delegator
    /// @param validatorSrcAddress The validator from which the redelegation is initiated
    /// @param validatorDstAddress The validator to which the redelegation is destined
    /// @param amount The amount of the bond denomination to be redelegated to the validator
    /// This amount should use the bond denomination precision stored in the bank metadata.
    /// @return success Whether or not the redelegate was successfully enqueued
    function wrappedRedelegate(
        address delegatorAddress,
        address validatorSrcAddress,
        address validatorDstAddress,
        uint256 amount
    ) external returns (bool success);

    /// @dev Allows delegators to cancel the unbondingDelegation entry
    /// and to delegate back to a previous validator.
    /// @param delegatorAddress The address of the delegator
    /// @param validatorAddress The address of the validator
    /// @param amount The amount of the bond denomination
    /// This amount should use the bond denomination precision stored in the bank metadata.
    /// @param creationHeight The height at which the unbonding took place
    /// @return success Whether or not the unbonding delegation was cancelled
    function wrappedCancelUnbondingDelegationBech32(
        address delegatorAddress,
        string memory validatorAddress,
        uint256 amount,
        uint256 creationHeight
    ) external returns (bool success);

    /// @dev Allows delegators to cancel the unbondingDelegation entry
    /// and to delegate back to a previous validator.
    /// @param delegatorAddress The address of the delegator
    /// @param validatorAddress The address of the validator
    /// @param amount The amount of the bond denomination
    /// This amount should use the bond denomination precision stored in the bank metadata.
    /// @param creationHeight The height at which the unbonding took place
    /// @return success Whether or not the unbonding delegation was cancelled
    function wrappedCancelUnbondingDelegation(
        address delegatorAddress,
        address validatorAddress,
        uint256 amount,
        uint256 creationHeight
    ) external returns (bool success);

    /// @dev Queries the given amount of the bond denomination to a validator.
    /// @param delegatorAddress The address of the delegator.
    /// @param validatorAddress The address of the validator.
    /// @return shares The amount of shares, that the delegator has received.
    /// @return balance The amount in Coin, that the delegator has delegated to the given validator.
    /// This returned balance uses the bond denomination precision stored in the bank metadata.
    function delegationBech32(
        address delegatorAddress,
        string memory validatorAddress
    ) external view returns (uint256 shares, Coin calldata balance);

    /// @dev Queries the given amount of the bond denomination to a validator.
    /// @param delegatorAddress The address of the delegator.
    /// @param validatorAddress The address of the validator.
    /// @return shares The amount of shares, that the delegator has received.
    /// @return balance The amount in Coin, that the delegator has delegated to the given validator.
    /// This returned balance uses the bond denomination precision stored in the bank metadata.
    function delegation(
        address delegatorAddress,
        address validatorAddress
    ) external view returns (uint256 shares, Coin calldata balance);

    /// @dev Returns the delegation shares and coins, that are currently
    /// unbonding for a given delegator and validator pair.
    /// @param delegatorAddress The address of the delegator.
    /// @param validatorAddress The address of the validator.
    /// @return unbondingDelegation The delegations that are currently unbonding.
    function unbondingDelegationBech32(
        address delegatorAddress,
        string memory validatorAddress
    )
    external
    view
    returns (UnbondingDelegationOutputBech32 calldata unbondingDelegation);

    /// @dev Returns the delegation shares and coins, that are currently
    /// unbonding for a given delegator and validator pair.
    /// @param delegatorAddress The address of the delegator.
    /// @param validatorAddress The address of the validator.
    /// @return unbondingDelegation The delegations that are currently unbonding.
    function unbondingDelegation(
        address delegatorAddress,
        address validatorAddress
    )
    external
    view
    returns (UnbondingDelegationOutput calldata unbondingDelegation);

    /// @dev Queries validator info for a given validator address.
    /// @param validatorAddress The address of the validator.
    /// @return validator The validator info for the given validator address.
    function validatorBech32(
        address validatorAddress
    ) external view returns (ValidatorBech32 calldata validator);

    /// @dev Queries validator info for a given validator address.
    /// @param validatorAddress The address of the validator.
    /// @return validator The validator info for the given validator address.
    function validator(
        address validatorAddress
    ) external view returns (Validator calldata validator);

    /// @dev Queries all validators that match the given status.
    /// @param status Enables to query for validators matching a given status.
    /// @param pageRequest Defines an optional pagination for the request.
    function validatorsBech32(
        string memory status,
        PageRequest calldata pageRequest
    )
    external
    view
    returns (
        ValidatorBech32[] calldata validators,
        PageResponse calldata pageResponse
    );

    /// @dev Queries all validators that match the given status.
    /// @param status Enables to query for validators matching a given status.
    /// @param pageRequest Defines an optional pagination for the request.
    function validators(
        string memory status,
        PageRequest calldata pageRequest
    )
    external
    view
    returns (
        Validator[] calldata validators,
        PageResponse calldata pageResponse
    );

    /// @dev Queries all redelegations from a source to a destination validator for a given delegator.
    /// @param delegatorAddress The address of the delegator.
    /// @param srcValidatorAddress Defines the validator address to redelegate from.
    /// @param dstValidatorAddress Defines the validator address to redelegate to.
    /// @return redelegation The active redelegations for the given delegator, source and destination
    /// validator combination.
    function redelegationBech32(
        address delegatorAddress,
        string memory srcValidatorAddress,
        string memory dstValidatorAddress
    ) external view returns (RedelegationOutputBech32 calldata redelegation);

    /// @dev Queries all redelegations from a source to a destination validator for a given delegator.
    /// @param delegatorAddress The address of the delegator.
    /// @param srcValidatorAddress Defines the validator address to redelegate from.
    /// @param dstValidatorAddress Defines the validator address to redelegate to.
    /// @return redelegation The active redelegations for the given delegator, source and destination
    /// validator combination.
    function redelegation(
        address delegatorAddress,
        address srcValidatorAddress,
        address dstValidatorAddress
    ) external view returns (RedelegationOutput calldata redelegation);

    /// @dev Queries all redelegations based on the specified criteria:
    /// for a given delegator and/or origin validator address
    /// and/or destination validator address
    /// in a specified pagination manner.
    /// @param delegatorAddress The address of the delegator (can be a zero address).
    /// @param srcValidatorAddress Defines the validator address to redelegate from (can be empty string).
    /// @param dstValidatorAddress Defines the validator address to redelegate to (can be empty string).
    /// @param pageRequest Defines an optional pagination for the request.
    /// @return response Holds the redelegations for the given delegator, source and destination validator combination.
    function redelegationsBech32(
        address delegatorAddress,
        string memory srcValidatorAddress,
        string memory dstValidatorAddress,
        PageRequest calldata pageRequest
    )
    external
    view
    returns (
        RedelegationResponseBech32[] calldata response,
        PageResponse calldata pageResponse
    );

    /// @dev Queries all redelegations based on the specified criteria:
    /// for a given delegator and/or origin validator address
    /// and/or destination validator address
    /// in a specified pagination manner.
    /// @param delegatorAddress The address of the delegator (can be zero address).
    /// @param srcValidatorAddress Defines the validator address to redelegate from (can be zero address).
    /// @param dstValidatorAddress Defines the validator address to redelegate to (can be zero address).
    /// @param pageRequest Defines an optional pagination for the request.
    /// @return response Holds the redelegations for the given delegator, source and destination validator combination.
    function redelegations(
        address delegatorAddress,
        address srcValidatorAddress,
        address dstValidatorAddress,
        PageRequest calldata pageRequest
    )
    external
    view
    returns (
        RedelegationResponse[] calldata response,
        PageResponse calldata pageResponse
    );

    /// @dev Queries epoch information for a given epoch number.
    /// @param epochNumber The epoch number to query.
    /// @return response The epoch information.
    function epochInfo(
        uint64 epochNumber
    ) external view returns (EpochResponse calldata response);

    /// @dev Queries current epoch information.
    /// @return response The current epoch information.
    function currentEpoch() external view returns (CurrentEpochResponse calldata response);

    /// @dev Queries messages queued in a specific epoch.
    /// @param epochNumber The epoch number to query.
    /// @param pageRequest Pagination request parameters.
    /// @return response Array of queued messages, pageResponse Pagination response.
    function epochMsgs(
        uint64 epochNumber,
        PageRequest calldata pageRequest
    ) external view returns (
        QueuedMessageResponse[] calldata response,
        PageResponse calldata pageResponse
    );

    /// @dev Queries messages from the latest epochs.
    /// @param endEpoch The ending epoch number.
    /// @param epochCount Number of epochs to query backwards from endEpoch.
    /// @param pageRequest Pagination request parameters.
    /// @return response Array of epoch message lists, pageResponse Pagination response.
    function latestEpochMsgs(
        uint64 endEpoch,
        uint64 epochCount,
        PageRequest calldata pageRequest
    ) external view returns (
        QueuedMessageList[] calldata response,
        PageResponse calldata pageResponse
    );

    /// @dev Queries the lifecycle of a validator.
    /// @param validatorAddress The validator address to query.
    /// @return response Array of validator lifecycle events.
    function validatorLifecycle(
        address validatorAddress
    ) external view returns (
        ValidatorUpdateResponse[] calldata response
    );

    /// @dev Queries the lifecycle of delegations for a delegator.
    /// @param delegatorAddress The delegator address to query.
    /// @return response The delegation lifecycle information.
    function delegationLifecycleBech32(
        address delegatorAddress
    ) external view returns (
        DelegationLifecycleBech32 calldata response
    );

    /// @dev Queries the lifecycle of delegations for a delegator.
    /// @param delegatorAddress The delegator address to query.
    /// @return response The delegation lifecycle information.
    function delegationLifecycle(
        address delegatorAddress
    ) external view returns (
        DelegationLifecycle calldata response
    );

    /// @dev Queries the validator set for a specific epoch.
    /// @param epochNumber The epoch number to query.
    /// @param pageRequest Pagination request parameters.
    /// @return validators Array of validators in the epoch, totalVotingPower Total voting power of all validators, pageResponse Pagination response.
    function epochValSet(
        uint64 epochNumber,
        PageRequest calldata pageRequest
    ) external view returns (
        SimpleValidator[] calldata validators,
        int64 totalVotingPower,
        PageResponse calldata pageResponse
    );

    /// @dev WrappedCreateValidator defines an Event emitted when a create a new validator.
    /// @param validatorAddress The address of the validator
    /// @param value The amount of coin being self delegated
    event WrappedCreateValidator(address indexed validatorAddress, uint256 value);

    /// @dev WrappedEditValidator defines an Event emitted when edit a validator.
    /// @param validatorAddress The address of the validator.
    /// @param commissionRate The commission rate.
    /// @param minSelfDelegation The min self delegation.
    /// @param epochBoundary The epoch boundary when the delegation will be processed
    event WrappedEditValidator(
        address indexed validatorAddress,
        int256 commissionRate,
        int256 minSelfDelegation,
        uint64 epochBoundary
    );

    /// @dev WrappedDelegate defines an Event emitted when a given amount of tokens are delegated from the
    /// delegator address to the validator address.
    /// @param delegatorAddress The address of the delegator
    /// @param validatorAddress The address of the validator
    /// @param amount The amount of bond denomination being delegated
    /// This amount has the bond denomination precision stored in the bank metadata.
    /// @param newShares The new delegation shares being held
    /// @param epochBoundary The epoch boundary when the delegation will be processed
    event WrappedDelegate(
        address indexed delegatorAddress,
        address indexed validatorAddress,
        uint256 amount,
        uint256 newShares,
        uint64 epochBoundary
    );

    /// @dev WrappedUnbond defines an Event emitted when a given amount of tokens are unbonded from the
    /// validator address to the delegator address.
    /// @param delegatorAddress The address of the delegator
    /// @param validatorAddress The address of the validator
    /// @param amount The amount of bond denomination being unbonded
    /// This amount has the bond denomination precision stored in the bank metadata.
    /// @param epochBoundary The epoch boundary when the unbonding will be processed
    event WrappedUnbond(
        address indexed delegatorAddress,
        address indexed validatorAddress,
        uint256 amount,
        uint64 epochBoundary
    );

    /// @dev WrappedRedelegate defines an Event emitted when a given amount of tokens are redelegated from
    /// the source validator address to the destination validator address.
    /// @param delegatorAddress The address of the delegator
    /// @param validatorSrcAddress The address of the validator from which the delegation is retracted
    /// @param validatorDstAddress The address of the validator to which the delegation is directed
    /// @param amount The amount of bond denomination being redelegated
    /// This amount has the bond denomination precision stored in the bank metadata.
    /// @param epochBoundary The epoch boundary when the redelegation will be processed
    event WrappedRedelegate(
        address indexed delegatorAddress,
        address indexed validatorSrcAddress,
        address indexed validatorDstAddress,
        uint256 amount,
        uint64 epochBoundary
    );

    /// @dev CancelUnbondingDelegation defines an Event emitted when a given amount of tokens
    /// that are in the process of unbonding from the validator address are bonded again.
    /// @param delegatorAddress The address of the delegator
    /// @param validatorAddress The address of the validator
    /// @param amount The amount of bond denomination that was in the unbonding process which is to be canceled
    /// This amount has the bond denomination precision stored in the bank metadata.
    /// @param creationHeight The block height at which the unbonding of a delegation was initiated
    /// @param epochBoundary The epoch boundary when the cancel unbonding will be processed
    event WrappedCancelUnbondingDelegation(
        address indexed delegatorAddress,
        address indexed validatorAddress,
        uint256 amount,
        uint256 creationHeight,
        uint64 epochBoundary
    );
}
