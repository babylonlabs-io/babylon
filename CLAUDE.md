# Babylon: Bitcoin Security for the Decentralized Economy

## Overview

Babylon is a Bitcoin-based security infrastructure that provides trustless security-sharing protocols between Bitcoin and Proof-of-Stake (PoS) blockchains. The project unlocks Bitcoin's 21 million BTC to secure decentralized systems without requiring Bitcoin holders to bridge their assets elsewhere.

**Key Value Propositions:**
- **Bitcoin Timestamping**: Submit verifiable timestamps of any data (such as PoS blockchains) to Bitcoin
- **Bitcoin Staking**: Enable Bitcoin holders to provide economic security to decentralized systems through trustless, self-custodial staking

## Architecture

Babylon is built using the Cosmos SDK and consists of several interconnected modules that work together to provide Bitcoin-secured infrastructure:

### Core Modules

#### **Epoching** (`x/epoching`)
- Divides the Babylon blockchain into epochs consisting of parameterized blocks
- Validator set remains unchanged within each epoch
- Reduces checkpointing costs by requiring only one checkpoint per epoch instead of per block
- Delays validator set changes to the last block of each epoch

#### **BTC Light Client** (`x/btclightclient`)
- Maintains a Bitcoin header chain based on Bitcoin's Proof-of-Work rules
- Receives Bitcoin headers from Vigilante Reporter
- Provides canonical Bitcoin chain information and header depth
- Validates inclusion evidence for Bitcoin transactions

#### **BTC Checkpoint** (`x/btccheckpoint`)
- Verifies Babylon's Bitcoin checkpoints reported by Vigilante Reporter
- Provides confirmation status based on checkpoint depth according to BTC Light Client
- Ensures Bitcoin-secured finality for Babylon blocks

#### **Checkpointing** (`x/checkpointing`)
- Creates Babylon checkpoints for submission to Bitcoin
- Collects BLS signatures from validators for each block to be checkpointed
- Aggregates signatures into BLS multisignatures for Bitcoin checkpoint inclusion
- Manages checkpoint confirmation status

#### **BTC Staking** (`x/btcstaking`)
- Core bookkeeping module for the Bitcoin staking protocol
- Manages finality providers and BTC delegations
- Handles staking requests, covenant signatures, and unbonding
- Maintains active finality provider set and voting power distribution

#### **Finality** (`x/finality`)
- Provides finality on top of CometBFT consensus
- Receives and verifies finality votes from finality providers
- Uses Extractable One-Time Signatures (EOTS) for finality voting
- Determines block finalization based on Bitcoin stake-weighted voting power

#### **Zone Concierge** (`x/zoneconcierge`)
- Extracts verified Consumer Zone headers from IBC light clients
- Maintains Bitcoin confirmation status for Consumer Zone transactions
- Communicates Bitcoin confirmation status through IBC connections
- Enables Bitcoin-secured checkpointing for other blockchains

#### **Incentive** (`x/incentive`)
- Distributes rewards to Bitcoin stakers and vigilantes
- Consumes percentage of Babylon staker rewards for Bitcoin stakers
- Manages reward distribution mechanisms

### Supporting Modules

- **BTC Staking Consumer** (`x/btcstkconsumer`): Manages consumer chain registrations
- **Monitor** (`x/monitor`): Provides monitoring capabilities
- **Mint** (`x/mint`): Handles token minting with custom logic

## Bitcoin Staking Protocol

### Key Actors

1. **BTC Stakers (Delegators)**: Bitcoin holders who delegate their BTC to finality providers
2. **Finality Providers**: Entities that receive Bitcoin delegations and participate in finality voting
3. **Covenant Emulators**: Committee members who enforce spending conditions on staked Bitcoin
4. **Vigilantes**: Relayers that monitor and report Bitcoin/Babylon state synchronization

### Staking Process

#### Post-Staking Registration
1. Create Bitcoin staking transaction with timelock and slashing conditions
2. Construct pre-signed slashing and unbonding transactions
3. Submit staking transaction to Bitcoin
4. Send delegation info to Babylon via `MsgCreateBTCDelegation`
5. Covenant committee verifies and signs transactions
6. Delegation becomes active

#### Pre-Staking Registration
1. Construct transactions and submit to Babylon first (without inclusion proof)
2. Covenant committee verifies and signs
3. Staker submits transaction to Bitcoin after confirmation
4. Submit inclusion proof via `MsgAddBTCDelegationInclusionProof`
5. Delegation becomes active

### Security Model

- **Slashing**: Finality providers can be slashed for equivocation
- **Extractable One-Time Signatures (EOTS)**: Enables extraction of private keys upon double-signing
- **Covenant Committee**: Multi-signature committee that enforces protocol rules
- **Bitcoin Finality**: All finality decisions are ultimately secured by Bitcoin's security

## Peripheral Infrastructure

### Vigilante Suite
- **Vigilante Submitter**: Submits Babylon checkpoints to Bitcoin
- **Vigilante Reporter**: Reports Bitcoin headers and checkpoints to Babylon
- **Checkpointing Monitor**: Monitors consistency between Bitcoin and Babylon
- **BTC Staking Monitor**: Monitors staking transaction execution and slashing

### Consumer Zone Integration
- **IBC Relayer**: Maintains IBC connections between Babylon and consumer zones
- **Babylon Contract**: CosmWasm smart contract for consumer zone deployment
- Enables Bitcoin checkpointing without invasive consumer zone changes

## Development & Operations

### Build Requirements
- Go 1.23+
- Standard Cosmos SDK development environment

### System Requirements (Production)
- Quad Core AMD/Intel (amd64) CPU
- 32GB RAM
- 1TB NVMe Storage
- 100MBps bidirectional internet

### Key Commands
```bash
make build          # Build babylond binary
make install        # Install to system directories
babylond --help     # View available commands
```

### Testing & Validation
- Comprehensive test suites for all modules
- End-to-end testing infrastructure
- Bitcoin mainnet/testnet integration testing

## Protocol Features

### Bitcoin Integration
- **Native Bitcoin Scripts**: Uses Bitcoin's native scripting for staking contracts
- **No Bridges**: Bitcoin never leaves the Bitcoin network
- **SPV Proofs**: Uses Bitcoin SPV proofs for transaction verification
- **Timelock Security**: Leverages Bitcoin's timelock mechanisms

### Cosmos Ecosystem Integration
- **IBC Compatible**: Full Inter-Blockchain Communication protocol support
- **CosmWasm Support**: Smart contract capabilities for enhanced functionality
- **SDK Modules**: Standard Cosmos SDK module architecture
- **Governance**: On-chain governance for parameter updates

### Cryptographic Primitives
- **BLS Signatures**: Efficient signature aggregation for checkpointing
- **Schnorr Signatures**: Bitcoin-native signature scheme
- **Adaptor Signatures**: Enable extractable signatures for slashing
- **EOTS**: Extractable One-Time Signatures for finality voting

## Documentation Structure

- `/docs/`: High-level design and architecture documentation
- `/x/*/README.md`: Detailed module-specific documentation
- `proto/`: Protocol buffer definitions
- `docs.babylonlabs.io`: Comprehensive user-facing documentation

## Repository Structure

```
babylon/
├── app/                    # Application-level code and configuration
├── cmd/babylond/          # CLI binary implementation
├── x/                     # Cosmos SDK modules
│   ├── btcstaking/       # Bitcoin staking protocol
│   ├── finality/         # Finality provider voting
│   ├── checkpointing/    # Bitcoin checkpoint management
│   └── ...               # Other protocol modules
├── proto/                # Protocol buffer definitions
├── testutil/             # Testing utilities and helpers
├── types/                # Common types and utilities
├── btcstaking/           # Bitcoin staking utilities
└── crypto/               # Cryptographic primitives
```

This repository implements the core Babylon blockchain infrastructure, providing Bitcoin-secured consensus and staking capabilities for the broader Cosmos ecosystem.