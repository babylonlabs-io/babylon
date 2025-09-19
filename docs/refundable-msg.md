# Refundable Messages

The Babylon genesis has always refunded messages which 
is the part of the protocol to work correctly. Below is
the list of refundable messages:

- MsgInsertHeaders
- MsgInsertBTCSpvProof
- MsgAddCovenantSigs
- MsgBTCUndelegate
- MsgSelectiveSlashingEvidence
- MsgAddBTCDelegationInclusionProof
- MsgAddFinalitySig

## Current Tx Fee Refund Logic

We track `RefundableMsgCount` during processing messages at 
each msgServer and use it in the post handler of [RefundTxDecorator](../x/incentive/keeper/refund_tx_decorator.go).
Since we only have to refund tx fee for refundable messages, checking the given message 
is refundable happens in `CheckTxAndClearIndex` function of `RefundTxDecorator`.

There are three conditions for the given tx fee is refunded:

- every message in the given tx is refundable
- there are no duplicates in the given tx
- specifically for `MsgAddFinalitySig`, we don't want to refund tx fee
  in case of tx doesn't fail for not reverting state transition such as slashing
  (Refer to parts comparing appHash and `HasEvidence` of [`AddFinalitySig`](../x/finality/keeper/msg_server.go))

We can achieve the first condition by using `isRefundTx()` function inside `CheckTxAndClearIndex`.
For condition 2, such cases don't happen because every msgServer already
rejects duplicate messages inside the logic.

| message type | duplication checks |
| --- | --- |
| MsgInsertHeaders (x/btclightclient) | https://github.com/babylonlabs-io/babylon/blob/8cb9862b1d0631edd15df1039085afb4274823a9/x/btclightclient/types/btc_light_client.go#L375-L379 |
| MsgAddFinalitySig (x/finality) | https://github.com/babylonlabs-io/babylon/blob/0a9997843ef04e8ed8cddc46237c2f39dc7c3243/x/finality/keeper/msg_server.go#L127-L133 |
| MsgInsertBTCSpvProof (x/btccheckpoint) | https://github.com/babylonlabs-io/babylon/blob/8cb9862b1d0631edd15df1039085afb4274823a9/x/btccheckpoint/keeper/msg_server.go#L42-L44 |
| MsgAddBTCDelegationInclusionProof (x/btcstaking) | https://github.com/babylonlabs-io/babylon/blob/8cb9862b1d0631edd15df1039085afb4274823a9/x/btcstaking/keeper/inclusion_proof.go#L112-L115 |
| MsgAddCovenantSigs (x/btcstaking) | https://github.com/babylonlabs-io/babylon/blob/e959112764233b20377d053dbdc08533c1693ebe/x/btcstaking/keeper/msg_server.go#L261-L266 |
| MsgBTCUndelegate (x/btcstaking) | https://github.com/babylonlabs-io/babylon/blob/e959112764233b20377d053dbdc08533c1693ebe/x/btcstaking/keeper/msg_server.go#L532-L534 |
| MsgSelectiveSlashingEvidence (x/btcstaking) | https://github.com/babylonlabs-io/babylon/blob/e959112764233b20377d053dbdc08533c1693ebe/x/btcstaking/keeper/msg_server.go#L670-L675 |

The last condition is achieved by tracking `RefundableMsgCount` in each msgServer.
We increment it by one when executing the tx and then check if the counter
during message execution has the same value with total messages the tx has.

### Caveat

To enhance performance of `RefundTxDecorator`, we eliminate the logic of checking duplicates
inside `CheckTxAndClearIndex` which used to hash given messages to compare each others.

Therefore, we need to ensure every refundable message has its own duplicate verification
logic inside its msgServer.