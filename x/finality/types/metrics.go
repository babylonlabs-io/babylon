package types

import (
	"github.com/cosmos/cosmos-sdk/telemetry"
	"github.com/hashicorp/go-metrics"
)

// performance oriented metrics measuring the execution time of each message
const (
	MetricsKeyCommitPubRandList      = "commit_pub_rand_list"
	MetricsKeyAddFinalitySig         = "add_finality_sig"
	MetricsKeyUnjailFinalityProvider = "unjail_finality_provider"
)

const (
	/* Metrics for monitoring block finalization status */

	// MetricsKeyLastHeight is the key of the gauge recording the last height
	// of the ledger
	MetricsKeyLastHeight = "last_height"
	// MetricsKeyLastFinalizedHeight is the key of the gauge recording the
	// last height finalized by finality providers
	MetricsKeyLastFinalizedHeight = "last_finalized_height"

	/* Metrics for monitoring finality provider liveness */

	// MetricsKeyJailedFinalityProviderCounter is the total number of finality providers
	// that are labeled as jailed
	MetricsKeyJailedFinalityProviderCounter = "jailed_finality_provider_counter"
	// MetricsKeyUnjailedFinalityProviderCounter is the total number of finality providers
	// that are unjailed
	// the number of finality providers that are being jailed can be calculated by
	// jailed_finality_provider_counter - unjailed_finality_provider_counter
	MetricsKeyUnjailedFinalityProviderCounter = "unjailed_finality_provider_counter"
)

// RecordLastHeight records the last height. It is triggered upon `IndexBlock`
func RecordLastHeight(height uint64) {
	keys := []string{MetricsKeyLastHeight}
	labels := []metrics.Label{telemetry.NewLabel(telemetry.MetricLabelNameModule, ModuleName)}
	telemetry.SetGaugeWithLabels(
		keys,
		float32(height),
		labels,
	)
}

// RecordLastFinalizedHeight records the last finalized height. It is triggered upon
// finalizing a block becomes finalized
func RecordLastFinalizedHeight(height uint64) {
	keys := []string{MetricsKeyLastFinalizedHeight}
	labels := []metrics.Label{telemetry.NewLabel(telemetry.MetricLabelNameModule, ModuleName)}
	telemetry.SetGaugeWithLabels(
		keys,
		float32(height),
		labels,
	)
}

// IncrementJailedFinalityProviderCounter increments the counter for the jailed
// finality providers
func IncrementJailedFinalityProviderCounter() {
	keys := []string{MetricsKeyJailedFinalityProviderCounter}
	labels := []metrics.Label{telemetry.NewLabel(telemetry.MetricLabelNameModule, ModuleName)}
	telemetry.IncrCounterWithLabels(
		keys,
		1,
		labels,
	)
}

// IncrementUnjailedFinalityProviderCounter increments the counter for the unjailed
// finality providers
func IncrementUnjailedFinalityProviderCounter() {
	keys := []string{MetricsKeyUnjailedFinalityProviderCounter}
	labels := []metrics.Label{telemetry.NewLabel(telemetry.MetricLabelNameModule, ModuleName)}
	telemetry.IncrCounterWithLabels(
		keys,
		1,
		labels,
	)
}
