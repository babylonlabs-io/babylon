BTC Staking Consumer

The `btcstkconsumer` module is a core component of the Babylon protocol responsible for managing the registration and
lifecycle of external blockchains, known as consumers or BSNs (Bitcoin-Secured Networks). These BSNs leverage
Babylon's Bitcoin-staked security to enhance their own economic guarantees.

This module acts as a central registry, maintaining a canonical list of all BSNs and their metadata. It provides the
necessary logic for registering new BSNs, querying their information, and enforcing integration policies defined by 
the Babylon network's governance. All logic related to the Finality Providers (FPs) that secure these BSNs is 
handled by the `x/btcstaking` module.

## Table of Contents

1.  [Concepts](#1-concepts)
    *   [1.1. Consumer/BSN](#11-consumerbsn)
    *   [1.2. Consumer/BSN Registration](#12-consumerbsn-registration)
    *   [1.3. Consumer/BSN Types](#13-consumerbsn-types)
    *   [1.4. Finality Provider Management](#14-finality-provider-management)
    *   [1.5. Permissioned vs. Permissionless Integration](#15-permissioned-vs-permissionless-integration)
2.  [State](#2-state)
    *   [2.1. Parameters](#21-parameters)
    *   [2.2. Consumer Registry](#22-consumer-registry)
3.  [Messages](#3-messages)
    *   [3.1. `MsgRegisterConsumer`](#31-msgregisterconsumer)
    *   [3.2. `MsgUpdateParams`](#32-msgupdateparams)
4.  [BeginBlocker / EndBlocker](#4-beginblocker--endblocker)
5.  [Events](#5-events)
    *   [5.1. `EventConsumerRegistered`](#51-eventconsumerregistered)
6.  [Queries](#6-queries)
    *   [6.1. Parameters Queries](#61-parameters-queries)
    *   [6.2. Consumer Queries](#62-consumer-queries)

## 1. Concepts

### 1.1. Consumer/BSN

<!--TO DO: change all instances of consumer to bsn when code changed -->

A **Consumer/BSN** is any external blockchain or rollup that registers with Babylon to utilise its security services. The
`btcstkconsumer` module maintains a unique record for each BSN.

### 1.2. Consumer/BSN Registration

This is the process by which a new BSN is officially recognised by Babylon. The registration process
captures essential metadata about the BSN, which varies depending on its integration type. This process is initiated
via a `MsgRegisterConsumer` transaction.

### 1.3. Consumer/BSN Types

The module supports two primary types of BSN integrations:

#### 1.3.1. Cosmos Consumer/BSN

*   **Description**: A blockchain built with the Cosmos SDK that integrates with Babylon via an IBC channel.
*   **Identifier (`consumer_id`)**: The IBC Client ID (`e.g., 07-cometbft-0`) that Babylon uses to track the state of
    the BSN chain.
*   **Integration Logic**: The module verifies the existence of the specified IBC client on Babylon before completing 
    the registration.

#### 1.3.2. Rollup Consumer/BSN

*   **Description**: A rollup (e.g., an optimistic or ZK-rollup) that uses a smart contract on Babylon for its data
    availability or finality needs.
*   **Identifier (`consumer_id`)**: A unique, human-readable string that identifies the rollup (e.g., `my-rollup-chain-1`).
    Note: The consumer ID for rollup BSNs can be any unique string chosen arbitrarily.
*   **Integration Logic**: The registration requires the address of a `RollupFinalityContractAddress`,
    which must be a valid, deployed CosmWasm contract on the Babylon chain. The module verifies the contract's existence.

### 1.4. Finality Provider Management

The `x/btcstaking` module is solely responsible for managing the global set of Finality Providers (FPs), including both
Babylon's genesis FPs and those that secure consumer BSNs. It maintains the crucial mapping that links each FP to the
specific BSN it serves. This centralization ensures that all logic related to FP registration, rewards, and slashing
is handled in a single module.

The `btcstkconsumer` module does **not** store any information about finality providers; it only serves as a registry
for BSN metadata. All queries related to which FPs are securing a BSN, or which BSN an FP is securing, must be
directed to the `x/btcstaking` module.

### 1.5. Permissioned vs. Permissionless Integration

The module can operate in two modes, controlled by the `PermissionedIntegration` parameter:

*   **Permissionless (Default)**: Any user can submit a `MsgRegisterConsumer` transaction to register a new BSN,
    provided the integration requirements (e.g., existing IBC client or Wasm contract) are met.
*   **Permissioned**: New BSN registrations can only be executed through a governance proposal. The `signer` of
    the `MsgRegisterConsumer` must be the governance module's account address. This allows the Babylon DAO to curate
    which chains can connect.

## 2. State

The `btcstkconsumer` module persists the following objects in its state, using prefixed keys for organisation.

### 2.1. Parameters

The module's behavior is governed by a set of parameters, stored as a single object. The parameters are defined in
`x/btcstkconsumer/types/params.pb.go`.

*   **Store Key**: `types.ParamsKey ("p_btcstkconsumer")`
*   **Value**: Marshaled `Params` object.

```protobuf
// x/btcstkconsumer/types/params.pb.go
message Params {
    // permissioned_integration is a flag to enable permissioned integration, i.e.,
    // requiring governance proposal to approve new integrations.
    bool permissioned_integration = 1;
}
```

| Parameter               | Type   | Default | Description                                                     |
| :---------------------- | :----- | :------ | :-------------------------------------------------------------- |
| `PermissionedIntegration` | `bool` | `false` | If `true`, new BSN registration requires a governance proposal. |

### 2.2. Consumer Registry

A registry of all consumers that have been successfully onboarded to Babylon.

*   **State Object**: `types.ConsumerRegister`
*   **Store Key**: `types.ConsumerRegisterKey (0x01) | []byte(consumerID)`
*   **Value**: Marshaled `ConsumerRegister` object.

The `ConsumerRegister` object, defined in `x/btcstkconsumer/types/btcstkconsumer.pb.go`, captures the core information
about a BSN.

```protobuf
// x/btcstkconsumer/types/btcstkconsumer.pb.go
message ConsumerRegister {
  // consumer_id is the ID of the consumer
  // - for Cosmos SDK chains, the consumer ID will be the IBC client ID
  // - for rollup chains, the consumer ID will be the chain ID of the rollup
  //   chain
  string consumer_id = 1;
  // consumer_name is the name of the consumer
  string consumer_name = 2;
  // consumer_description is a description for the consumer (can be empty)
  string consumer_description = 3;
  // consumer_metadata is necessary metadata of the consumer, and the data
  // depends on the type of integration
  oneof consumer_metadata {
    CosmosConsumerMetadata cosmos_consumer_metadata = 4;
    RollupConsumerMetadata rollup_consumer_metadata = 5;
  };
}
```

## 3. Messages

The module exposes the following messages to trigger state transitions.

### 3.1. `MsgRegisterConsumer`

This message is used to register a new BSN chain with Babylon. It is defined in `x/btcstkconsumer/types/tx.pb.go`.

```protobuf
// x/btcstkconsumer/types/tx.pb.go
// MsgRegisterConsumer defines a message for registering Consumers to the
// btcstkconsumer module.
message MsgRegisterConsumer {
  option (cosmos.msg.v1.signer) = "signer";

  string signer = 1;
  // consumer_id is the ID of the consumer
  string consumer_id = 2;
  // consumer_name is the name of the consumer
  string consumer_name = 3;
  // consumer_description is a description for the consumer (can be empty)
  string consumer_description = 4;
  // rollup_finality_contract_address is the address of the
  // finality contract. The finality contract is deployed on Babylon and
  // serves as the data availability layer for finality signatures of the rollup.
  // (if set, then this means this is a rollup integration)
  string rollup_finality_contract_address = 5 [(cosmos_proto.scalar) = "cosmos.AddressString"];
}
```

**Validation & Logic:**

1.  The `ConsumerId`, `ConsumerName`, and `ConsumerDescription` fields cannot be empty.
2.  If `PermissionedIntegration` is `true`, the `Signer` must be the governance module's authority address.
3.  If `RollupFinalityContractAddress` is provided, the message is handled as a **Rollup Consumer/BSN**:
    *   The module verifies that the address corresponds to an existing CosmWasm contract.
4.  If `RollupFinalityContractAddress` is empty, it is handled as a **Cosmos Consumer/BSN**:
    *   The module verifies that `ConsumerId` corresponds to an existing IBC client state.
5.  It checks that the `ConsumerId` is not already registered to prevent duplicates.
6.  On success, it persists the `ConsumerRegister` object and emits an `EventConsumerRegistered`.

### 3.2. `MsgUpdateParams`

This message is used to update the module's parameters. It is a governance-gated operation. It is defined in
`x/btcstkconsumer/types/tx.pb.go`.

```protobuf
// x/btcstkconsumer/types/tx.pb.go
message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  option (amino.name) = "babylon/x/btcstkconsumer/v1/MsgUpdateParams";

  // authority is the address that controls the module (defaults to x/gov unless overwritten).
  string authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];

  // params defines the module parameters to update.
  //
  // NOTE: All parameters must be supplied.
  Params params = 2 [
    (gogoproto.nullable) = false,
    (amino.dont_omitempty) = true
  ];
}
```

**Validation & Logic:**

1.  The `Authority` field must match the module's configured authority address (the governance module account).
2.  The provided `Params` are validated.
3.  On success, the new parameters are persisted in the state.

## 4. BeginBlocker / EndBlocker

The `btcstkconsumer` module's `BeginBlocker` and `EndBlocker` currently do not contain any core state-transition logic.
Their primary function is for emitting telemetry data. This means that no automatic state changes for this module occur
at the beginning or end of a block.

## 5. Events

The module emits events upon successful execution of certain messages.

### 5.1. EventConsumerRegistered

This event is emitted after a new consumer/BSN is successfully registered via `MsgRegisterConsumer`. It is defined in
`x/btcstkconsumer/types/events.pb.go`.

```protobuf
// x/btcstkconsumer/types/events.pb.go
// EventConsumerRegistered is the event emitted when a consumer is registered
message EventConsumerRegistered {
  // consumer_id is the id of the consumer
  string consumer_id = 1  [(amino.dont_omitempty) = true];
  // consumer_name is the name of the consumer
  string consumer_name = 2  [(amino.dont_omitempty) = true];
  // consumer_description is a description for the consumer
  string consumer_description = 3  [(amino.dont_omitempty) = true];
  // consumer_type is the type of the consumer
  ConsumerType consumer_type = 4 [(amino.dont_omitempty) = true];
  // consumer_metadata is necessary metadata of the consumer, and the data
  // depends on the type of integration
  RollupConsumerMetadata rollup_consumer_metadata = 5;
}
```

*   `consumer_type`: Indicates whether the BSN is `COSMOS` (0) or `ROLLUP` (1).
*   `rollup_consumer_metadata`: This field is always present. For `ROLLUP` 
    consumers, it contains the finality contract address. For `COSMOS`
    consumers, it is present but its `finality_contract_address` field will be an empty string.
    Note: `rollup_consumer_metadata` will be empty and/or appear where the Cosmos metadata is shown in the event payload.

## 6. Queries

The module provides gRPC, REST, and CLI interfaces for querying its state.

### 6.1. Parameters Queries

Retrieves the current parameters of the module.

| Interface | Method/Endpoint/Command                  |
| :-------- | :--------------------------------------- |
| **gRPC**  | `babylon.btcstkconsumer.v1.Query.Params` |
| **REST**  | `GET /babylon/btcstkconsumer/v1/params`  |
| **CLI**   | `babylond query btcstkconsumer params`   |

### 6.2. Consumer Queries

#### List All Registered Consumers/BSNs

Returns a paginated list of all BSNs registered with Babylon.

| Interface | Method/Endpoint/Command                                |
| :-------- | :----------------------------------------------------- |
| **gRPC**  | `babylon.btcstkconsumer.v1.Query.ConsumerRegistryList` |
| **REST**  | `GET /babylon/btcstkconsumer/v1/consumer_registry_list`|
| **CLI**   | `babylond query btcstkconsumer registered-consumers`   |

#### Get a Specific Consumer/BSN's Information

Returns the registration details for one or more specified consumer IDs.

| Interface | Method/Endpoint/Command                                       |
| :-------- | :------------------------------------------------------------ |
| **gRPC**  | `babylon.btcstkconsumer.v1.Query.ConsumersRegistry`           |
| **REST**  | `GET /babylon/btcstkconsumer/v1/consumers_registry/{consumer_ids}` |
| **CLI**   | `babylond query btcstkconsumer registered-consumer <consumer-id>` |
