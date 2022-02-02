package metrics

import (
	"net/http"

	"github.com/Azure/azure-container-networking/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "npm"

// Prometheus Metrics
// Gauge metrics have the methods Inc(), Dec(), and Set(float64)
// Summary metrics have the method Observe(float64)
// For any Vector metric, you can call With(prometheus.Labels) before the above methods
//   e.g. SomeGaugeVec.With(prometheus.Labels{label1: val1, label2: val2, ...).Dec()
var (
	numPolicies        prometheus.Gauge
	addPolicyExecTime  prometheus.Summary
	numACLRules        prometheus.Gauge
	addACLRuleExecTime prometheus.Summary
	numIPSets          prometheus.Gauge
	addIPSetExecTime   prometheus.Summary
	numIPSetEntries    prometheus.Gauge
	ipsetInventory     *prometheus.GaugeVec

	policyApplyTime    *prometheus.SummaryVec
	podApplyTime       *prometheus.SummaryVec
	namespaceApplyTime *prometheus.SummaryVec

	execTimeQuantiles = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}
)

// Constants for metric names and descriptions as well as exported labels for Vector metrics
const (
	numPoliciesName = "num_policies"
	numPoliciesHelp = "The number of current network policies for this node"

	addPolicyExecTimeName = "add_policy_exec_time"
	addPolicyExecTimeHelp = "Execution time in milliseconds for adding a network policy"

	// TODO do update/delete
	policyApplyTimeName = "policy_apply_time"
	policyApplyTimeHelp = "Execution time in milliseconds for updating/deleting a network policy. NOTE: for apply time for adding, see add_policy_exec_time"
	applyModeLabel      = "apply_mode"

	podApplyTimeName = "pod_apply_time"
	podApplyTimeHelp = "Execution time in milliseconds for adding/updating/deleting a pod"

	namespaceApplyTimeName = "namespace_apply_time"
	namespaceApplyTimeHelp = "Execution time in milliseconds for adding/updating/deleting a namespace"

	numACLRulesName = "num_iptables_rules"
	numACLRulesHelp = "The number of current IPTable rules for this node"

	addACLRuleExecTimeName = "add_iptables_rule_exec_time"
	addACLRuleExecTimeHelp = "Execution time in milliseconds for adding an IPTable rule to a chain"

	numIPSetsName = "num_ipsets"
	numIPSetsHelp = "The number of current IP sets for this node"

	addIPSetExecTimeName = "add_ipset_exec_time"
	addIPSetExecTimeHelp = "Execution time in milliseconds for creating an IP set"

	numIPSetEntriesName = "num_ipset_entries"
	numIPSetEntriesHelp = "The total number of entries in every IPSet"

	ipsetInventoryName = "ipset_counts"
	ipsetInventoryHelp = "The number of entries in each individual IPSet"
	setNameLabel       = "set_name"
	setHashLabel       = "set_hash"
)

var (
	nodeLevelRegistry    = prometheus.NewRegistry()
	clusterLevelRegistry = prometheus.NewRegistry()
	haveInitialized      = false
)

type ApplyMode string

const (
	CreateMode ApplyMode = "create"
	UpdateMode ApplyMode = "update"
	DeleteMode ApplyMode = "delete"
)

var knownApplyModes = map[ApplyMode]struct{}{
	CreateMode: {},
	UpdateMode: {},
	DeleteMode: {},
}

// InitializeAll creates all the Prometheus Metrics. The metrics will be nil before this method is called.
func InitializeAll() {
	if !haveInitialized {
		numPolicies = createGauge(numPoliciesName, numPoliciesHelp, false)
		addPolicyExecTime = createSummary(addPolicyExecTimeName, addPolicyExecTimeHelp, true)
		numACLRules = createGauge(numACLRulesName, numACLRulesHelp, false)
		addACLRuleExecTime = createSummary(addACLRuleExecTimeName, addACLRuleExecTimeHelp, true)
		numIPSets = createGauge(numIPSetsName, numIPSetsHelp, false)
		addIPSetExecTime = createSummary(addIPSetExecTimeName, addIPSetExecTimeHelp, true)
		numIPSetEntries = createGauge(numIPSetEntriesName, numIPSetEntriesHelp, false)
		ipsetInventory = createGaugeVec(ipsetInventoryName, ipsetInventoryHelp, false, setNameLabel, setHashLabel)

		policyApplyTime = createSummaryVec(policyApplyTimeName, policyApplyTimeHelp, true, applyModeLabel)
		podApplyTime = createSummaryVec(podApplyTimeName, podApplyTimeHelp, true, applyModeLabel)
		namespaceApplyTime = createSummaryVec(namespaceApplyTimeName, namespaceApplyTimeHelp, true, applyModeLabel)

		log.Logf("Finished initializing all Prometheus metrics")
		haveInitialized = true
	}
}

// ReinitializeAll creates/replaces Prometheus metrics. This function is intended for UTs.
// Be sure to reset helper variables e.g. ipsetInventoryMap.
func ReinitializeAll() {
	haveInitialized = false
	InitializeAll()
	ipsetInventoryMap = make(map[string]int)
}

// GetHandler returns the HTTP handler for the metrics endpoint
func GetHandler(isNodeLevel bool) http.Handler {
	return promhttp.HandlerFor(getRegistry(isNodeLevel), promhttp.HandlerOpts{})
}

func register(collector prometheus.Collector, name string, isNodeLevel bool) {
	err := getRegistry(isNodeLevel).Register(collector)
	if err != nil {
		log.Errorf("Error creating metric %s", name)
	}
}

func getRegistry(isNodeLevel bool) *prometheus.Registry {
	registry := clusterLevelRegistry
	if isNodeLevel {
		registry = nodeLevelRegistry
	}
	return registry
}

func createGauge(name, helpMessage string, isNodeLevel bool) prometheus.Gauge {
	gauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      name,
			Help:      helpMessage,
		},
	)
	register(gauge, name, isNodeLevel)
	return gauge
}

func createGaugeVec(name, helpMessage string, isNodeLevel bool, labels ...string) *prometheus.GaugeVec {
	gaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      name,
			Help:      helpMessage,
		},
		labels,
	)
	register(gaugeVec, name, isNodeLevel)
	return gaugeVec
}

func createSummary(name, helpMessage string, isNodeLevel bool) prometheus.Summary {
	summary := prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace:  namespace,
			Name:       name,
			Help:       helpMessage,
			Objectives: execTimeQuantiles,
			// quantiles e.g. the "0.5 quantile" will actually be the phi quantile for some phi in [0.5 - 0.05, 0.5 + 0.05]
		},
	)
	register(summary, name, isNodeLevel)
	return summary
}

func createSummaryVec(name, helpMessage string, isNodeLevel bool, labels ...string) *prometheus.SummaryVec {
	summary := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  namespace,
			Name:       name,
			Help:       helpMessage,
			Objectives: execTimeQuantiles,
			// quantiles e.g. the "0.5 quantile" will actually be the phi quantile for some phi in [0.5 - 0.05, 0.5 + 0.05]
		},
		labels,
	)
	register(summary, name, isNodeLevel)
	return summary
}
