package dto

import (
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

const ProxyTypeHttp = "http"
const ProxyTypeSocks5 = "socks5"

var (
	InvalidProxyType          = errors.New("invalid proxy type")
	MissingProxyAddressOrPort = errors.New("missing proxy address or port")
)

type ClusterResourceModel struct {
	ID                       types.String     `tfsdk:"id"`
	Name                     types.String     `tfsdk:"name"`
	Description              types.String     `tfsdk:"description"`
	PolicyId                 types.String     `tfsdk:"policy_id"`
	ResourceId               types.String     `tfsdk:"resource_id"`
	ApiKey                   types.String     `tfsdk:"api_key"`
	Endpoint                 types.String     `tfsdk:"endpoint"`
	Orchestrator             types.String     `tfsdk:"orchestrator"`
	CreatedDateTime          types.String     `tfsdk:"created_date_time"`
	UpdatedDateTime          types.String     `tfsdk:"updated_date_time"`
	LastEvaluatedDateTime    types.String     `tfsdk:"last_evaluated_date_time"`
	GroupId                  types.String     `tfsdk:"group_id"`
	Namespaces               types.Set        `tfsdk:"namespaces"`
	RuntimeSecurityEnabled   types.Bool       `tfsdk:"runtime_security_enabled"`
	VulnerabilityScanEnabled types.Bool       `tfsdk:"vulnerability_scan_enabled"`
	InventoryCollection      types.Bool       `tfsdk:"inventory_collection"`
	Proxy                    ProxyDetailModel `tfsdk:"proxy"`
}

type ProxyDetailModel struct {
	Type         types.String `tfsdk:"type"`
	ProxyAddress types.String `tfsdk:"proxy_address"`
	Port         types.Int64  `tfsdk:"port"`
	Username     types.String `tfsdk:"username"`
	Password     types.String `tfsdk:"password"`
	HttpsProxy   types.String `tfsdk:"https_proxy"`
}

type PolicyResourceModel struct {
	ID                   types.String                    `tfsdk:"id"`
	Name                 types.String                    `tfsdk:"name"`
	Description          types.String                    `tfsdk:"description"`
	PolicyDefault        *PolicyDefaultResourceModel     `tfsdk:"default"`
	PolicyNamespacedList []PolicyNamespacedResourceModel `tfsdk:"namespaced"`
	PolicyRuntime        *PolicyRuntimeResourceModel     `tfsdk:"runtime"`
	XdrEnabled           types.Bool                      `tfsdk:"xdr_enabled"`

	CreatedDateTime         types.String `tfsdk:"created_date_time"`
	UpdatedDateTime         types.String `tfsdk:"updated_date_time"`
	RulesetsUpdatedDateTime types.String `tfsdk:"rulesets_updated_date_time"`
}

type PolicyDefaultResourceModel struct {
	PolicyRuleList      []PolicyRuleResourceModel `tfsdk:"rules"`
	PolicyExceptionList []PolicyRuleResourceModel `tfsdk:"exceptions"`
}

type PolicyRuntimeResourceModel struct {
	PolicyRulesetList []PolicyRulesetResourceModel `tfsdk:"rulesets"`
}

type PolicyRulesetResourceModel struct {
	ID types.String `tfsdk:"id"`
}

type PolicyNamespacedResourceModel struct {
	Name                types.String              `tfsdk:"name"`
	Namespaces          []types.String            `tfsdk:"namespaces"`
	PolicyRuleList      []PolicyRuleResourceModel `tfsdk:"rules"`
	PolicyExceptionList []PolicyRuleResourceModel `tfsdk:"exceptions"`
}

type PolicyRuleResourceModel struct {
	Type                types.String                      `tfsdk:"type"`
	Enabled             types.Bool                        `tfsdk:"enabled"`
	Action              types.String                      `tfsdk:"action"`
	Mitigation          types.String                      `tfsdk:"mitigation"`
	PolicyRuleStatement *PolicyRuleStatementResourceModel `tfsdk:"statement"`
}

type PolicyRuleStatementResourceModel struct {
	PolicyRulePropertyList []PolicyRulePropertyResourceModel `tfsdk:"properties"`
}

type PolicyRulePropertyResourceModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}
