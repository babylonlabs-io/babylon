package testnet

// CosmWasm parameters for testnet should allow everybody to
// upload and instantiate.
const CosmWasmParamStr = `{
  "code_upload_access": {
    "permission": "Everybody"
  },
  "instantiate_default_permission": "Everybody"
}`
