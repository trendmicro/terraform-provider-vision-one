package dto

import (
	"errors"
)

var (
	ErrorBadRequest = errors.New("bad request")
	ErrorNotFound   = errors.New("resource not found")
	ErrorForbidden  = errors.New("forbidden")
	ErrorInternal   = errors.New("internal error")
	Unauthorized    = errors.New("unauthorized access or invalid api key")
)

type CreateClusterResponse struct {
	ID       string `json:"id"`
	ApiKey   string `json:"apiKey"`
	Endpoint string `json:"endpointUrl"`
}

type ListClusterResponse struct {
	Items      []ClusterItem `json:"items"`
	TotalCount int           `json:"totalCount"`
	Count      int           `json:"count"`
}

type GetClusterResponse struct {
	Item ClusterItem
}

type ClusterItem struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	Description              string `json:"description"`
	RuntimeSecurityEnabled   bool   `json:"runtimeSecurityEnabled"`
	VulnerabilityScanEnabled bool   `json:"vulnerabilityScanEnabled"`
	MalwareScanEnabled       bool   `json:"malwareScanEnabled"`
	PolicyId                 string `json:"policyId"`
	Orchestrator             string `json:"orchestrator"`
	Nodes                    []Node `json:"nodes"`
	ResourceId               string `json:"resourceId"`
	CreatedDateTime          string `json:"createdDateTime"`
	UpdatedDateTime          string `json:"updatedDateTime"`
	LastEvaluatedDateTime    string `json:"lastEvaluatedDateTime"`
}

type Node struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Pods []Pod  `json:"pods"`
}

type Pod struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ListRulesetsResponse struct {
	Items []*RulesetResponse `json:"items"`
}

type RulesetResponse struct {
	Id              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Labels          []Label
	Rules           []Rule
	CreatedDateTime string `json:"createdDateTime"`
	UpdatedDateTime string `json:"updatedDateTime"`
}

// Container Security - Policy

type PolicyResponse struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Default     *PolicyDefault     `json:"default"`
	Namespaced  []PolicyNamespaced `json:"namespaced,omitempty"`
	Runtime     *PolicyRuntime     `json:"runtime,omitempty"`
	XdrEnabled  bool               `json:"xdrEnabled"`

	CreatedDateTime         string `json:"createdDateTime"`
	UpdatedDateTime         string `json:"updatedDateTime,omitempty"`
	RulesetsUpdatedDateTime string `json:"rulesetsUpdatedDateTime,omitempty"`

	MalwareScan *MalwareScan `json:"malwareScan,omitempty"`
}

type ListPolicyResponse struct {
	Items      []PolicyResponse `json:"items"`
	TotalCount int              `json:"totalCount"`
	Count      int              `json:"count"`
}
