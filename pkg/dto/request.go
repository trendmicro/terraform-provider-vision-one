package dto

type CreateClusterRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PolicyId    string `json:"policyId"`
	ResourceId  string `json:"resourceId"`
	GroupId     string `json:"groupId"`
}

type CreateRulesetRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Labels      []Label
	Rules       []Rule
}

type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Rule struct {
	Id         string `json:"id"`
	Enabled    bool   `json:"enabled"`
	Mitigation string `json:"mitigation"`
}

type DeleteClusterRequest struct {
	ID string
}

type GetClusterRequest struct {
	ID string
}

type UpdateClusterRequest struct {
	Description string `json:"description"`
	PolicyId    string `json:"policyId"`
	ResourceId  string `json:"resourceId"`
	GroupId     string `json:"groupId"`
}

// Container Security - Policy Request

type CreatePolicyRequest struct {
	Name                 string             `json:"name"`
	Description          string             `json:"description"`
	PolicyDefault        *PolicyDefault     `json:"default"`
	PolicyNamespacedList []PolicyNamespaced `json:"namespaced,omitempty"`
	PolicyRuntime        *PolicyRuntime     `json:"runtime,omitempty"`
	XdrEnabled           bool               `json:"xdrEnabled"`

	MalwareScan *MalwareScan `json:"malwareScan,omitempty"`
}

type MalwareScan struct {
	Mitigation *string   `json:"mitigation,omitempty"`
	Schedule   *Schedule `json:"schedule,omitempty"`
}

type Schedule struct {
	Enabled *bool   `json:"enabled,omitempty"`
	Cron    *string `json:"cron,omitempty"`
}

type PolicyRuntime struct {
	PolicyRulesetList []PolicyRuleset `json:"rulesets"`
}

type PolicyRuleset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PolicyNamespaced struct {
	Name                string       `json:"name,omitempty"`
	Namespaces          []string     `json:"namespaces,omitempty"`
	PolicyRuleList      []PolicyRule `json:"rules,omitempty"`
	PolicyExceptionList []PolicyRule `json:"exceptions,omitempty"`
}

type PolicyDefault struct {
	PolicyRuleList      []PolicyRule `json:"rules,omitempty"`
	PolicyExceptionList []PolicyRule `json:"exceptions,omitempty"`
}

type PolicyRule struct {
	Type                string               `json:"type,omitempty"`
	Enabled             bool                 `json:"enabled"`
	Action              string               `json:"action,omitempty"`
	Mitigation          string               `json:"mitigation,omitempty"`
	PolicyRuleStatement *PolicyRuleStatement `json:"statement,omitempty"`
}

type PolicyRuleStatement struct {
	PolicyRulePropertyList []PolicyRuleProperty `json:"properties,omitempty"`
}

type PolicyRuleProperty struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type UpdatePolicyRequest struct {
	Description          string             `json:"description,omitempty"`
	PolicyDefault        *PolicyDefault     `json:"default,omitempty"`
	PolicyNamespacedList []PolicyNamespaced `json:"namespaced,omitempty"`
	PolicyRuntime        *PolicyRuntime     `json:"runtime,omitempty"`
	XdrEnabled           bool               `json:"xdrEnabled"`

	MalwareScan *MalwareScan `json:"malwareScan,omitempty"`
}
