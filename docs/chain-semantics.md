# Babylon Genesis Chain Semantics

This document defines the chain semantics of the Babylon Genesis Chain it
outlines core properties, state transitions and modules.

---

## 1. Core Prroperties

### Token Model

- **Native Token**: `BABY`
- **Base Denomination**: `ubbn`
- **Exponent**: `10^6` (i.e., `1 BABY = 1,000,000 ubbn`)
- **Staking Tokens**: Both BABY and BTC

### Chain  Identity

- Babylon maintains a  specific `chain-id` (e.g., `bbn-1` for mainnet)
- Consumer chains integrated with Babylon are tracked using internal
`ConsumerID`s

> For network details, refer to the [Babylon networks](https://github.com/babylonlabs-io/networks) repository.

---

## 2. State Transitions

### BTC Delegation State Transition

Babylon enables non-custodial BTC staking. Delegations follow a defined state machine, with different flows for pre-registration and post-registration.

#### Delegation Lifecycle

There are two valid state transition paths for BTC delegations:

- `PENDING → VERIFIED → ACTIVE → UNBONDED → EXPIRED`
- `PENDING → VERIFIED → ACTIVE → UNBONDED/EXPIRED`

These two separate paths for delegating:
**Post-Registration Flow:**

- `PENDING → ACTIVE`
  - Bitcoin transaction already confirmed at registration time
  - VERIFIED state is skipped as Bitcoin confirmation already exists

**Pre-Registration Flow:**

- `PENDING → VERIFIED → ACTIVE`
  - `PENDING`: Waiting for covenant signatures
  - `VERIFIED`: Covenant signatures obtained, waiting for Bitcoin confirmation
  - ACTIVE: Becomes active once Bitcoin confirmation is deep enough

**Invalid Path:**

- `PENDING → VERIFIED → UNBONDED` is considered invalid
  - Occurs if a delegation is unbonded before reaching the `ACTIVE` state

#### State Definitions

| State        | Description                                         |
| ------------ | --------------------------------------------------- |
| **PENDING**  | Waiting for covenant signatures                     |
| **VERIFIED** | Covenant signed, but not confirmed on Bitcoin       |
| **ACTIVE**   | Confirmed on Bitcoin, has voting power              |
| **UNBONDED** | No longer has power, either undelegated or unbonded |
| **EXPIRED**  | Timelock expired, delegation naturally concluded    |

Defined in [btcstaking.proto](../proto/babylon/btcstaking/v1/btcstaking.proto).

### Finality Provider State Transitions

Finality providers have their own lifecycle as documented in
[events.proto](../proto/babylon/finality/v1/events.proto).

**State Transitions:**

1. `INACTIVE` → `ACTIVE`: When sufficient delegations and timestamped public
    randomness
2. `ACTIVE` → `JAILED`: Due to downtime
3. `ACTIVE` → `SLASHED`': Due to double-signing
4. `JAILED` → `INACTIVE/ACTIVE`: After unjailing period (depending on conditions)
5. `ACTIVE` → `INACTIVE`: When insufficient delegations or no timestamped randomness

**Note**: `SLASHED` finality providers cannot transition to other states.

### Finality Voting

Finality voting is an additional consensus round that runs ontop of CometBFT
consensus, where finality providers vote on blocks using voting power derived
from Bitcoin delegations. These votes are submitted using Extractable One-Time
Signatures (EOTS).

#### Block Finalisation Lifecycle

The finality voting process follows this lifecycle:

1. CometBFT commits a block to the ledger
2. Finality providers submit EOTS signatures via `MsgAddFinalitySig`
3. System tallies votes during EndBlocker (implemented in abci.go:21-28)
4. Block becomes finalized if it receives >2/3 voting power

#### EOTS Signatures

Finality votes use Extractable One-Time Signatures (EOTS) to ensure
cryptographic safety. If a finality provider signs conflicting blocks at the
same height, their secret key can be extracted for slashing.

The EOTS process requires:

- Public Randomness Commitment: Finality providers proactively commit EOTS
public randomness for future heights
- Signatures: Upon new blocks, providers submit EOTS signatures using the
committed randomness

Voting power is assigned to the top 100 finality providers with BTC timestamped
public randomness, ranked by total delegated value. Babylon updates the voting
power distribution at every `BeginBlock` by processing events from the BTC
staking module.

Blocks must be finalised sequentially meaning that the blocks must be in a
strict order of finalisation from the earliest non-finalised height. If block
`N` doesnt receive enough votes to be finalised then blocks `N+1` and `N+2`
cannot be finalised either, even with sufficient votes.

#### Slashing

When finality providers vote for blocks at the same height babylon will verify
if the voted block is a fork or not if it does then evidence is created and the
provider is slashed by setting their voting power to zero and extracting their
BTC PK (private key).

#### Voting Conditions

A finality provider _must_ submit a vote if:

- The block is not yet finalised.
- The provider has voting power at that height.
- The provider has not already voted at that height.
- All prior unfinalised blocks with voting power have been signed.

#### Statuses

- Gains power → `ACTIVE`
- Loses power → `INACTIVE`
- Double-signs → `SLASHED` (permanently)

## Checkpoint State Transitions

Checkpoints progress through Bitcoin confirmation states as defined in:

- `ACCUMULATING`: Collecting BLS signatures from validators
- `SEALED`: Has sufficient BLS signatures (>2/3 voting power)
- `SUBMITTED`: At least one known submission on BTC main chain
- `CONFIRMED`: At least one submission that is k-deep
- `FINALIZED`: Exactly one submission that is w-deep

### Message-Driven State Transitions

The state transitions are triggered by specific messages:

**BTC Staking Flow:**

1. `MsgCreateBTCDelegation` → `PENDING` - This creates a `BTCDelegation`
    object `PENDING` state (waiting for covenant signatures)
2. `MsgAddCovenantSigs` → `VERIFIED` - This adds signatures to the delegation,
    transitioning it to `VERIFIED` state once quorum is reached
3. `MsgAddBTCDelegationInclusionProof` → `ACTIVE` - This message transitions
    the delegation from `VERIFIED` to `ACTIVE` state, giving it voting power
4. `MsgBTCUndelegate` → `UNBONDED` - This message transitions the delegation to `UNBONDED` state, removing its voting power

**Finality Provider Flow:**

1. `MsgCreateFinalityProvider` →  Creates and stores a `FinalityProvider` 
object in the system
2. `MsgCommitPubRandList` → Makes the finality provider eligible to receive
voting power and participate in consensus
3. `MsgAddFinalitySig` → Helping achieve >2/3 voting power threshold needed to
finalise blocks, in other words, finality voting

These state transitions are enforced through validation logic in the respective
message handlers.

---

## 3. Core Modules

Babylon introduces custom Cosmos SDK modules:

- `x/btcstaking`: Enables native, non-custodial BTC staking
- `x/checkpointing`: Aggregates and timestamps consumer checkpoints
- `x/finality`: Collects finality votes from providers
- `x/zoneconcierge`: Accepts headers from IBC-connected consumer chains
- `x/btclightclient`: Verifies Bitcoin blocks for checkpoint inclusion
- `x/epoching`: Manages fixed-length epochs for batching validator and
checkpoint updates

---

## 4. IBC

IBC is the communication layer between the Babylon chain and consumer chains.
The Zone Concierge module serves as the central IBC coordinator, managing all
cross-chain interactions through defined packet types and channels.

### Packets

The IBC protocol defines two distinct packet categories :

**Outbound Packet Types** (Babylon → Consumer Chains):

- `BTCTimestamp` which carries Bitcoin finalised epoch information including
headers, checkpoints, and cryptographic proofs
- `BTCStakingIBCPacket` which transmits BTC staking state updates and finality
provider information
- `BTCHeaders` that delivers verified Bitcoin headers for consumer  the chain
light client

**Inbound Packet Types** (Consumer Chains → Babylon):

- `ConsumerSlashingIBCPacket` which delivers slashing evidence from consumer
chains for finality provider accountability

### Channels

Channels identify the specific connection between two chains and specific
rules are enforced during the IBC handshake:

Channel Requirements:

- Only `ORDERED` channels are permitted to ensure sequential packet delivery
- Channel port must match the Zone Concierge bound port
- Protocol version must match the expected Zone Concierge version
- Client ID must correspond to a registered Cosmos consumer

Channel Restrictions:

- User created channel closures are prohibited
- Channels can only be closed through protocol-level events
- Each consumer chain maintains exactly one active channel
