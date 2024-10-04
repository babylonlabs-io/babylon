package mainnet

// CosmWasm parameters for mainnet should allow only the
// governance module account authtypes.NewModuleAddress(govtypes.ModuleName)
// to upload and everybody to instantiate.
// AccessTypeEverybody AccessType = 3
// AccessTypeAnyOfAddresses AccessType = 4
const CosmWasmParamStr = `{
  "code_upload_access": {
    "permission": "ACCESS_TYPE_ANY_OF_ADDRESSES",
    "addresses": ["bbn10d07y265gmmuvt4z0w9aw880jnsr700jduz5f2"]
  },
  "instantiate_default_permission": "ACCESS_TYPE_EVERYBODY"
}`
