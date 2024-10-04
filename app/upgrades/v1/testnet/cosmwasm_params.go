package testnet

// CosmWasm parameters for testnet should allow everybody to
// upload and instantiate.
const CosmWasmParamStr = `{
  "code_upload_access": {
    "permission": "ACCESS_TYPE_EVERYBODY",
    "addresses": []
  },
  "instantiate_default_permission": "ACCESS_TYPE_EVERYBODY"
}`
