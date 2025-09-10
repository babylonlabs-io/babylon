package mainnet

// CosmWasm parameters for mainnet should allow only the
// governance module account authtypes.NewModuleAddress(govtypes.ModuleName)
// to upload and everybody to instantiate.
const CosmWasmParamStr = `{
  "code_upload_access": {
    "permission": "Everybody"
  },
  "instantiate_default_permission": "Everybody"
}`
