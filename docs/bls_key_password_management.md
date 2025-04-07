# BLS Key Management in Babylon

This document describes how BLS keys are managed in Babylon, including password
management options and command-line flags for secure key generation and usage.

## Overview

BLS keys in Babylon are used for validator signatures in the checkpointing
mechanism. These keys are stored encrypted using the ERC-2335 keystore format
and require a password for decryption when the node starts.

## Password Management Options

There are three main ways to provide the password for the BLS key:

1. **Environment Variable (Recommended)**: Set the `BABYLON_BLS_PASSWORD` 
   environment variable
2. **Password File**: Create a file containing only the password and provide
   its path using the `--bls-password-file` flag
3. **Interactive Prompt**: If no password is provided through environment variable
   or file, you will be prompted to enter it interactively

## Key Generation

When generating a new BLS key (during `init` or using `create-bls-key`), you
can provide a password through:

1. Environment variable `BABYLON_BLS_PASSWORD`
2. Password file specified with `--bls-password-file`
3. Interactive prompt (if neither of the above are provided)

For interactive prompts, the input will be hidden, and you'll need to confirm your
password by entering it twice. If the passwords don't match, you'll have up to 3
attempts to enter matching passwords before the command fails.

### Command Line Flags

The following command-line flags can be used with `babylond init`, 
`babylond create-bls-key`, and `babylond start`:

- `--no-bls-password` - Generate key without password protection (suitable for 
  non-validator nodes)
- `--bls-password-file` - Specify a path to a file containing or to store the password

Example:
```bash
# Create a BLS key with no password (for RPC nodes)
babylond create-bls-key --no-bls-password

# Create a BLS key and store the password in a file
babylond create-bls-key --bls-password-file="/path/to/password.txt"

# Start a node using a password from a custom location
babylond start --bls-password-file="/path/to/password.txt"
```

## Using Environment Variables (Recommended)

Setting an environment variable is the recommended approach for production
environments as it avoids storing the password in plaintext on disk.

```bash
# Set the environment variable
export BABYLON_BLS_PASSWORD="your-secure-password"

# Start the Babylon node
babylond start
```

## Creating and Using Password Files

If you prefer to store the password in a file:

1. Create a text file containing only your password
2. Set appropriate permissions (e.g., `chmod 600`)
3. Provide the path when starting the node:

```bash
# Create a password file
echo "your-secure-password" > /path/to/bls_password.txt
chmod 600 /path/to/bls_password.txt

# Start node with password file
babylond start --bls-password-file="/path/to/bls_password.txt"
```

When creating a new key with `--bls-password-file`, the password will be stored in the
specified file for you.

## Password Options

When loading or creating a BLS key, Babylon gives the following options:

1. If `--no-bls-password` is specified, an empty password is used regardless of other settings
2. `BABYLON_BLS_PASSWORD` environment variable
3. Password file specified with `--bls-password-file`
4. Interactive prompt (for new keys: with confirmation flow; for existing keys: single prompt)

Important: Multiple password methods cannot be used simultaneously. The system will 
validate that only one method is provided and throw an error if multiple flags are passed.

## Security Considerations

1. **Do not use empty passwords** for validator nodes. Only use the
   `--no-bls-password` flag for non-validator nodes.

2. **Store passwords securely**. You are responsible for remembering
   or securely storing your password.

3. **Consider using a password manager** to generate and store your BLS key
   password securely.

4. **Environment variables are generally more secure** than storing passwords
   in files, as they exist only in memory rather than on disk.

## Non-Validator Nodes

For non-validator nodes (like RPC nodes) that don't need to sign checkpoints,
you can use the `--no-bls-password` flag to avoid password management
completely:

```bash
# Start a non-validator node with no BLS password
babylond start --no-bls-password
```

## Viewing BLS Key Information

You can view information about your BLS key using the `show-bls-key` command:

```bash
# View BLS key information using environment variable for password
babylond show-bls-key

# View BLS key information using password file
babylond show-bls-key --bls-password-file="/path/to/password.txt"

# View BLS key information for a key with no password
babylond show-bls-key --no-bls-password
```

The command will display the public key and other key information in JSON format.

## Backing Up Your BLS Key

It is highly recommended to securely back up your BLS key file after creation. 
Losing access to your BLS key can prevent your validator from participating in the network.
Store the backup in a safe place different from the default configuration folder of your validator node.
