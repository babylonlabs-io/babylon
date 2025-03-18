# BLS Key Management in Babylon

This document describes how BLS keys are managed in Babylon, including password
management options and command-line flags for secure key generation and usage.

## Overview

BLS keys in Babylon are used for validator signatures in the checkpointing
mechanism. These keys are stored encrypted using the ERC-2335 keystore format
and require a password for decryption when the node starts.

## Password Management Options

There are three ways to provide the password for the BLS key:

1. **Environment Variable (Recommended)**: Set the `BABYLON_BLS_PASSWORD` 
   environment variable
2. **Password File**: Store the password in a file (default location: 
   `$HOME/.babylond/config/bls_password.txt` or custom location)
3. **Interactive Prompt**: Enter the password when prompted during node startup

## Key Generation

When generating a new BLS key (during `init` or using `create-bls-key`), you'll
be prompted to choose how to manage your password:

```
Select the storage strategy for your BLS password.
1. Environment variable (recommended)
2. Local file (not recommended)
```

### Command Line Flags

The following command-line flags can be used with `babylond init`, 
`babylond create-bls-key`, and `babylond start`:

- `--bls-password` - Directly specify the password (not recommended for 
  production)
- `--no-bls-password` - Generate key without password protection (suitable for 
  non-validator nodes)
- `--bls-password-file` - Specify a custom path for the password file

Example:
```bash
# Create a BLS key with no password (for RPC nodes)
babylond create-bls-key --no-bls-password

# Create a BLS key with a specified password
babylond create-bls-key --bls-password="your-secure-password"

# Start a node using a password from a custom location
babylond start --bls-password-file="/path/to/custom/password.txt"
```

## Using Environment Variables (Recommended)

Using environment variables is the recommended approach for production
environments as it avoids storing the password in plaintext on disk.

```bash
# Set the environment variable
export BABYLON_BLS_PASSWORD="your-secure-password"

# Start the Babylon node
babylond start
```

**Important**: When using the environment variable method, Babylon will not 
write the password to any file, enhancing security by avoiding plaintext
storage on disk.

## Using Password Files

For deployment scenarios where environment variables aren't suitable, Babylon
supports reading the password from a file.

Default location:
```
$HOME/.babylond/config/bls_password.txt
```

Custom location (specified via flag):
```bash
babylond start --bls-password-file="/path/to/custom/password.txt"
```

## Priority Order

When loading the BLS key, Babylon checks for the password in the following 
order:

1. Direct password provided via `--bls-password` flag
2. `BABYLON_BLS_PASSWORD` environment variable
3. Custom password file path (if specified with `--bls-password-file`)
4. Default password file path (`$HOME/.babylond/config/bls_password.txt`)
5. Interactive prompt (if none of the above are available)

If `--no-bls-password` is specified, the system will use an empty password
regardless of other settings.

## Non-Validator Nodes

For non-validator nodes (like RPC nodes) that don't need to sign checkpoints,
you can use the `--no-bls-password` flag to avoid password management
completely:

```bash
# Start a non-validator node with no BLS password
babylond start --no-bls-password
```
