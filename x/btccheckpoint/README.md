# BTCCheckpoint

The BTC Checkpoint module is responsible for receiving and managing the checkpoints of Babylon's state on the Bitcoin blockchain. This includes:
- handling requests for submitting raw checkpoints
- processing Bitcoin SPV proofs for submitted checkpoints
- managing the lifecycle of checkpoints (SEALED, SUBMITTED, CONFIRMED, FINALIZED)
- handling the verification and finalization of checkpoints
- distributing rewards for successful checkpoint submissions, and
- proactively updating the status of checkpoints based on Bitcoin blockchain confirmations

## Table of contents

- [Table of contents](#table-of-contents)
- [Concepts](#concepts)
- [States](#states)
  - [Parameters](#parameters)
  - [Epoch data](#epoch-data)
  - [Latest Finalized Epoch](#latest-finalized-epoch)
  - [Submission data](#submission-data)
  - [Transient States](#transient-states)
- [Messages](#messages)
  - [MsgInsertBTCSpvProof](#msginsertbtcspvproof)
  - [MsgUpdateParams](#msgupdateparams)
- [EndBlocker](#endblocker)
- [Queries](#queries)

## Concepts 
Babylon's BTC Checkpoint module allows Babylon chain to periodically checkpoint its state onto the Bitcoin blockchain whilst simultaneously verifying the checkpoints. The process involves two main components:

Checkpoint Submission:
1. When a new checkpoint for a specific epoch is available, the [vigilante](https://github.com/babylonlabs-io/vigilante) collects the necessary proof from the Bitcoin blockchain and submits raw transactions containing the checkpoint to the Babylon chain.
2. This proof is called an `SPVProof` (Simplified Payment Verification Proof), consisting of the Bitcoin transaction, Index of the transaction, the Merkle path and the Bitcoin header that confirms the transaction.
3. The [vigilante reporter](https://github.com/babylonlabs-io/vigilante/blob/47956edbb72112162e4cecca5b9d1e0ad840dd47/reporter/utils.go#L191) submits this proof to the Babylon chain using the `InsertBTCSpvProof` function. The proof is validated and stored in state. This includes parsing the submission, checking for duplicates, and verifying the checkpoint with the [checkpointing module](https://github.com/babylonlabs-io/babylon/blob/main/x/checkpointing/README.md).

Checkpoint Verification:
1. The Babylon chain maintains a Bitcoin light client through the [BTC Light Client module](https://github.com/babylonlabs-io/babylon/blob/dev/x/btclightclient/README.md). This module is responsible for tracking Bitcoin block headers, allowing Babylon to verify the depth and validity of Bitcoin transactions without running a full Bitcoin node.
2. When new Bitcoin blocks are produced, the headers are relayed to and processed by the Babylon chain's light client.
3. As the light client's tip changes, it triggers the `OnTipChange` callback.
4. This callback initiates the `checkCheckpoints` process, which verifies the status of all submitted checkpoints based on their confirmation depth in the Bitcoin blockchain. This process includes:

   * Checking if the checkpoint is still on the canonical chain
   * Verifying if the deepest (best) submission of an older epoch happened
     before the given submission
   * Determining how deep the submission is in the blockchain
   * Marking each submission for deletion if it is:
     - Not known to the BTC light client, or
     - On a fork of the BTC light client's chain

   For more details on submissions, see [Submissions](#submission-data).
5. Non-finalized epochs are retrieved from state. For each of these non-finalized epochs, the status is checked of the corresponding checkpoint. The depth of the checkpoint in the Bitcoin blockchain is verified and based on the depth and the module's parameters, the checkpoint's status may be updated. If the status changed, it's updated in the state and the corresponding status is set in the checkpointing module. Following an epoch being finalized, all submissions except the best one are deleted.

## States 

The BTC Checkpoint module uses a combination of prefixed namespaces and individual keys within its KV store to organize different types of data. This approach allows for efficient storage and retrieval of various data elements related to checkpoints, epochs, and submissions.

**Prefixes**

- `SubmisionKeyPrefix` is used to prefix keys for storing submission data.

- `EpochDataPrefix` is used to prefix keys for storing epoch-related data.

**Keys**

- `LastFinalizedEpochKey` stores the number of the last finalized epoch

- `BtcLightClientUpdatedKey` indicates whether the BTC light client was updated during the current block execution.

- `ParamsKey` stores modules parameters.

### Parameters
The [parameter management](https://github.com/babylonlabs-io/babylon/blob/main/x/btccheckpoint/keeper/params.go) maintains the BTC Checkpoint module's parameters. The BTC Checkpoint module's parameters are represented as a `Params` [object](https://github.com/babylonlabs-io/babylon/blob/main/proto/babylon/btccheckpoint/v1/params.proto) defined as follows:

```protobuf
// Params defines the parameters for the module.
message Params {
  option (gogoproto.equal) = true;

  // btc_confirmation_depth is the confirmation depth in BTC.
  // A block is considered irreversible only when it is at least k-deep in BTC
  // (k in research paper)
  uint32 btc_confirmation_depth = 1
      [ (gogoproto.moretags) = "yaml:\"btc_confirmation_depth\"" ];

  // checkpoint_finalization_timeout is the maximum time window (measured in BTC
  // blocks) between a checkpoint
  // - being submitted to BTC, and
  // - being reported back to BBN
  // If a checkpoint has not been reported back within w BTC blocks, then BBN
  // has dishonest majority and is stalling checkpoints (w in research paper)
  uint32 checkpoint_finalization_timeout = 2
      [ (gogoproto.moretags) = "yaml:\"checkpoint_finalization_timeout\"" ];

  // 4byte tag in hex format, required to be present in the OP_RETURN transaction
  // related to babylon
  string checkpoint_tag = 3
      [ (gogoproto.moretags) = "yaml:\"checkpoint_tag\"" ];
}
```

### Epoch Data

Epoch data is managed by [submissions management](https://github.com/babylonlabs-io/babylon/blob/main/x/btccheckpoint/keeper/submissions.go) and is used to store and retrieve epoch-related data. The epoch data is indexed by epoch number and is represented as an `EpochData` object:

```protobuf
message EpochData {
  // keys is the list of all received checkpoints during this epoch, sorted by
  // order of submission.
  repeated SubmissionKey keys = 1;

  // status is the current btc status of the epoch
  BtcStatus status = 2;
}
```

### Latest Finalized Epoch

The Last Finalized Epoch number is stored in the state as a big-endian encoded uint64 value. It's accessed and modified using specific getter and setter functions in the keeper.

### Submission Data

The [submissions management](https://github.com/babylonlabs-io/babylon/blob/main/x/btccheckpoint/keeper/submissions.go) is responsible for managing and interacting with checkpoint submissions in the BTC checkpoint module The `SubmissionData` is defined as an object below.

```protobuf
message SubmissionData {
  // address of the submitter and reporter
  CheckpointAddresses vigilante_addresses = 1;
  // txs_info is the two `TransactionInfo`s corresponding to the submission
  // It is used for
  // - recovering address of sender of btc transaction to payup the reward.
  // - allowing the ZoneConcierge module to prove the checkpoint is submitted to
  // BTC
  repeated TransactionInfo txs_info = 2;
  uint64 epoch = 3;
}
```

### Transient States

### BTC Light Client Update

The BTC Light Client Update is maintained in the transient store during block execution. It is accessed using the `BtcLightClientUpdatedKey` and indicates whether the BTC light client was updated during the current block execution.

```go
func GetBtcLightClientUpdatedKey() []byte {
	return BtcLightClientUpdatedKey
}
```

## Messages

The BTC Checkpoint module primarily handles messages from the vigilante reporter. The message formats are defined in [proto/babylon/btccheckpoint/v1/tx.proto](proto/babylon/btccheckpoint/v1/tx.proto.). The message handlers are defined in [x/btccheckpoint/keeper/msg_server.go](x/btccheckpoint/keeper/msg_server.go). For more information on the SDK messages, refer to the [Cosmos SDK documentation on messages and queries](https://docs.cosmos.network/main/build/building-modules/messages-and-queries)

### MsgInsertBTCSpvProof

`MsgInsertBTCSpvProof` is used by vigilante reporter to insert a new checkpoint into the store, which can be seen [here](https://github.com/babylonlabs-io/vigilante/blob/24da0381465249aa7b55be682a66e32cdaddc81b/types/btccheckpoint.go#L11). 

```protobuf
message MsgInsertBTCSpvProof {
  option (cosmos.msg.v1.signer) = "submitter";
  string submitter = 1;
  repeated babylon.btccheckpoint.v1.BTCSpvProof proofs = 2;
}
```

Upon receiving a `MsgInsertBTCSpvProof`, a Babylon node will execute as follows:

1. Parse and validate the raw checkpoint data from the proof.
2. Create a new `RawCheckpointSubmission` object from the parsed data.
3. Verify if the submission is for the expected checkpoint by calling the checkpointing keeper.
4. Check if the checkpoint is valid for the current epoch.
5. Verify the ancestors of the checkpoint to ensure continuity.
6. If all verifications pass, the new checkpoint submission is stored and the checkpoint status is updated.

## MsgUpdateParams

This message is used to update the `btccheckpoint` module parameters. This should only be executable through governance proposals.

```protobuf
message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";

  // authority is the address of the governance account.
  // just FYI: cosmos.AddressString marks that this field should use type alias
  // for AddressString instead of string, but the functionality is not yet implemented
  // in cosmos-proto
  string authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];

  // params defines the btccheckpoint parameters to update.
  //
  // NOTE: All parameters must be supplied.
  Params params = 2 [(gogoproto.nullable) = false];
}
```

## EndBlocker

Upon EndBlock, the BTC Checkpoint module executes the following:
- Check if the BTC light client head has been updated during the block execution using the `BtcLightClientUpdated` method.
- If the head has been updated, non-finalized epochs are checked to determine if their checkpoints have become confirmed, finalized, or abandoned.
The logic for the `EndBlocker` is defined in at [x/btccheckpoint/abci.go](https://github.com/babylonlabs-io/babylon/blob/main/x/btccheckpoint/abci.go).

## Queries

The BTC Checkpoint module provides a set of queries related to the status of checkpoints and other checkpoint-related data. These queries can be accessed via gRPC and REST endpoints.

### Available Queries

**Parameters**\
Endpoint: `/babylon/btccheckpoint/v1/params`\
Description: Queries the current parameters of the BTC Checkpoint module.

**BTC Checkpoint Info**\
Endpoint: `/babylon/btccheckpoint/v1/{epoch_num}`\
Description: Retrieves the best checkpoint information for a given epoch.

**BTC Checkpoints Info**\
Endpoint: `/babylon/btccheckpoint/v1/`\
Description: Retrieves checkpoint information for multiple epochs with pagination support.

**Epoch Submissions**\
Endpoint: `/babylon/btccheckpoint/v1/{epoch_num}/submissions`\
Description: Retrieves all submissions for a given epoch.

Additional Information: For further details on how to use these queries and additional documentation, please refer to docs.babylonlabs.io.