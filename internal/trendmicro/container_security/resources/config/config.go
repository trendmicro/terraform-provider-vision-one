package config

const (
	RESOURCE_TYPE_CLUSTER = "containersecurity_cluster"
	RESOURCE_TYPE_POLICY  = "containersecurity_policy"
	RESOURCE_TYPE_RULESET = "containersecurity_ruleset"

	RESOURCE_TYPE_CLUSTER_DESCRIPTION = "The `" + RESOURCE_TYPE_CLUSTER + "` resource allows you to manage Kubernetes cluster."
	RESOURCE_TYPE_POLICY_DESCRIPTION  = "The `" + RESOURCE_TYPE_POLICY + "` resource allows you to manage policies which define the rules that are used to control what is allowed to run in your Kubernetes cluster."
	RESOURCE_TYPE_RULESET_DESCRIPTION = "The `" + RESOURCE_TYPE_RULESET + "` resource allows you to manage several managed rules provided by Trend Micro to define a set of rules that you want to enforce for runtime security."
)
