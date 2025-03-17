# BLS Key Management in Babylon

This document describes how BLS keys are managed in Babylon, including the new environment variable feature 
for secure password management.

## Password Management

There are three ways to provide the password for the BLS key:

1. **Environment Variable (Recommended)**: Set the `BABYLON_BLS_PASSWORD` environment variable
2. **Password File (Not Recommended)**: Store the password in plain text in a file (default: `bls_password.txt`)
3. **Interactive Prompt (Not Recommended)**: Enter the password when prompted during node startup

### Using Environment Variables (Recommended)

Using environment variables is the recommended approach for production environments as it avoids storing
the password in plaintext on disk.

```bash
# Set the environment variable
export BABYLON_BLS_PASSWORD="your-secure-password"

# Start the Babylon node
babylond start
```

**Important**: When using the environment variable method, Babylon will not write the password to the 
password file, enhancing security by avoiding plaintext storage on disk.

### Using Password Files (Legacy)

For backward compatibility, Babylon still supports reading the password from a file. This method is not recommended
for production environments.

The password file is stored at:
```
$HOME/.babylond/config/bls_password.txt
```

When the environment variable is not set, Babylon will write the password to this file for future use.

### Priority

When loading the BLS key, Babylon checks for the password in the following order:

1. `BABYLON_BLS_PASSWORD` environment variable
2. Password file (`bls_password.txt`)

If the environment variable is set, it will be used regardless of whether the password file exists.
