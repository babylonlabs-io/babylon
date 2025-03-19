# BLS Key Management in Babylon

This document describes how BLS keys are managed in Babylon, including password
management options and command-line flags for secure key generation and usage.

## Overview

BLS keys in Babylon are used for validator signatures in the checkpointing
mechanism. These keys are stored encrypted using the ERC-2335 keystore format
and require a password for decryption when the node starts.

## Password Management Options

There are two main ways to provide the password for the BLS key:

1. **Environment Variable (Recommended)**: Set the `BABYLON_BLS_PASSWORD` 
   environment variable
2. **Password File**: Create a file containing only the password and provide
   its path when starting the node

## Key Generation

When generating a new BLS key (during `init` or using `create-bls-key`), you
will be prompted to enter a secure password. For security, the input will be
hidden, and you'll need to confirm your password by entering it twice.

If the passwords don't match, you'll have up to 3 attempts to enter matching
passwords before the command fails.

After key generation, the password is NOT automatically stored anywhere. You
are responsible for remembering or securely storing this password for future
use.

### Command Line Flags

The following command-line flags can be used with `babylond init`, 
`babylond create-bls-key`, and `babylond start`:

- `--insecure-bls-password` - Directly specify the password (not recommended for 
  production)
- `--no-bls-password` - Generate key without password protection (suitable for 
  non-validator nodes)
- `--bls-password-file` - Specify a path to a file containing the password

Example:
```bash
# Create a BLS key with no password (for RPC nodes)
babylond create-bls-key --no-bls-password

# Create a BLS key with a specified password
babylond create-bls-key --insecure-bls-password="your-secure-password"

# Start a node using a password from a custom location
babylond start --bls-password-file="/path/to/custom/password.txt"
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

## Priority Order

When loading the BLS key, Babylon checks for the password in the following 
order:

1. Direct password provided via `--insecure-bls-password` flag
2. `BABYLON_BLS_PASSWORD` environment variable
3. Password file specified with `--bls-password-file`
4. Interactive prompt

If `--no-bls-password` is specified, the system will use an empty password
regardless of other settings.

## Security Considerations

1. **Do not use empty passwords** for validator nodes. Only use the
   `--no-bls-password` flag for non-validator nodes.

2. **Store passwords securely**. Once the key is generated, the system will
   not store the password automatically. You are responsible for remembering
   or securely storing it.

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
