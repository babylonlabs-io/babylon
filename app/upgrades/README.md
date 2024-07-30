# Babylon Upgrades

This folder contains sub-folders for every babylon upgrade. (Both state
migrations, and hard forks) It also defines upgrade & hard fork structs,
that each upgrade implements. These then get included in the application
app.go to run the upgrade.

The code is adapted from [Osmosis](https://github.com/osmosis-labs/osmosis/blob/68d546d94acbf1aa99d6b514cb66b2b40afff4a4/app/upgrades).

## Version History

TODO

## Upgrade types

There are two upgrade types exposed, `Upgrade` and `Fork`. An `Upgrade`
defines an upgrade that is to be acted upon by state migrations from the
SDK `x/upgrade` module. A `Fork` defines a hard fork that changes some
logic at a block height. If the goal is to have a new binary be
compatible with the old binary prior to the upgrade height, as is the
case for all babylon `Fork`s, then all logic changes must be
height-gated or in the `BeginForkLogic` code.

```go
type Upgrade struct {
 // Upgrade version name, for the upgrade handler, e.g. `v7`
 UpgradeName string
 // Function that creates an upgrade handler
 CreateUpgradeHandler func(mm *module.Manager, configurator module.Configurator, keepers *keepers.AppKeepers) upgradetypes.UpgradeHandler
 // Store upgrades, should be used for any new modules introduced, new modules deleted, or store names renamed.
 StoreUpgrades store.StoreUpgrades
}

type Fork struct {
 // Upgrade version name, for the upgrade handler, e.g. `v7`
 UpgradeName string
 // height the upgrade occurs at
 UpgradeHeight int64

 // Function that runs some custom state transition code at the beginning of a fork.
 BeginForkLogic func(ctx sdk.Context, keepers *keepers.AppKeepers)
}
```
