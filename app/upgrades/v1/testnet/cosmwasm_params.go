package testnet

// CosmWasm parameters for testnet should allow everybody to
// upload and instantiate.
// For testnet everyone should be able to deploy CosmWasm contracts
// and stress test the most of the babylon node.
const CosmWasmParamStr = `{
  "code_upload_access": {
    "permission": "Everybody"
  },
  "instantiate_default_permission": "Everybody"
}`
