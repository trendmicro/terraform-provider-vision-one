package azure

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/trend-vcs/terraform-provider-vision-one/internal/trendmicro"
	"github.com/trend-vcs/terraform-provider-vision-one/internal/trendmicro/cloud_log_monitoring/azure/api"
	"github.com/trend-vcs/terraform-provider-vision-one/internal/trendmicro/cloud_log_monitoring/azure/resources/config"
)

var (
	_ resource.Resource              = &udcEventHubInfoResource{}
	_ resource.ResourceWithConfigure = &udcEventHubInfoResource{}
)

// ImportState is deliberately not implemented. The backend exposes no lookup a customer's own
// device token can call, so there is nothing an import could fetch — the same reason Read below
// is a no-op.

var guidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$`)

func NewAzureUdcEventHubInfoResource() resource.Resource {
	return &udcEventHubInfoResource{}
}

type udcEventHubInfoResource struct {
	client *api.ClmClient
}

// udcEventHubInfoModel mirrors one backend record: a subscription's whole Event Hub stack, not a
// single hub. The backend keys on tenant, subscription and region, so enabling a vendor changes
// event_hub_namespaces in place rather than adding another resource.
type udcEventHubInfoModel struct {
	ID                 types.String     `tfsdk:"id"`
	TenantID           types.String     `tfsdk:"tenant_id"`
	SubscriptionID     types.String     `tfsdk:"subscription_id"`
	Region             types.String     `tfsdk:"region"`
	ResourceGroup      types.String     `tfsdk:"resource_group"`
	EventHubNamespaces []namespaceModel `tfsdk:"event_hub_namespaces"`
	DeviceToken        types.String     `tfsdk:"scm_device_token"`
}

type namespaceModel struct {
	Name      types.String    `tfsdk:"name"`
	EventHubs []eventHubModel `tfsdk:"event_hubs"`
}

type eventHubModel struct {
	Name           types.String   `tfsdk:"name"`
	ConsumerGroups []types.String `tfsdk:"consumer_groups"`
}

func (m *udcEventHubInfoModel) toAPI() *api.UdcEventHubInfo {
	namespaces := make([]api.Namespace, 0, len(m.EventHubNamespaces))
	for _, ns := range m.EventHubNamespaces {
		hubs := make([]api.EventHub, 0, len(ns.EventHubs))
		for _, hub := range ns.EventHubs {
			groups := make([]string, 0, len(hub.ConsumerGroups))
			for _, cg := range hub.ConsumerGroups {
				groups = append(groups, cg.ValueString())
			}
			hubs = append(hubs, api.EventHub{Name: hub.Name.ValueString(), ConsumerGroups: groups})
		}
		namespaces = append(namespaces, api.Namespace{Name: ns.Name.ValueString(), EventHubs: hubs})
	}

	region := m.Region.ValueString()

	return &api.UdcEventHubInfo{
		CloudProvider:  api.CloudProviderAzure,
		CloudTenantID:  m.TenantID.ValueString(),
		CloudAccountID: m.SubscriptionID.ValueString(),
		// The backend keeps these separate against a future CLM region that no longer
		// inherits from CAM; today both are the CAM region.
		CloudRegion:            region,
		CloudParentStackRegion: region,
		Details: api.Details{
			ResourceGroup:      m.ResourceGroup.ValueString(),
			EventHubNamespaces: namespaces,
		},
	}
}

// computeID derives the record's id the same way the backend derives its own storage key, since
// the backend never hands one back: registering a stack returns only a human-readable message.
// Tenant and subscription are lowercased because the backend normalizes and keys on them
// case-insensitively.
func (m *udcEventHubInfoModel) computeID() types.String {
	return types.StringValue(fmt.Sprintf(
		"azure#TENANT#%s#SUB#%s#%s",
		strings.ToLower(m.TenantID.ValueString()),
		strings.ToLower(m.SubscriptionID.ValueString()),
		m.Region.ValueString(),
	))
}

func (r *udcEventHubInfoResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.RESOURCE_TYPE_AZURE_UDC_EVENTHUB_INFO
}

func (r *udcEventHubInfoResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	guid := func(attr string) validator.String {
		return stringvalidator.RegexMatches(guidPattern, attr+" must be a GUID")
	}

	// The record is keyed on tenant, subscription and region, so changing any of them
	// addresses a different row and cannot be an in-place update.
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: config.RESOURCE_TYPE_AZURE_UDC_EVENTHUB_INFO_DESCRIPTION,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier of the Event Hub stack record, derived from tenant_id, subscription_id and region.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tenant_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Azure tenant ID that owns the Event Hub namespaces.",
				Validators:          []validator.String{guid("tenant_id")},
				PlanModifiers:       replace,
			},
			"subscription_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Azure subscription ID containing the Event Hub namespaces. Trend Vision One uses it to obtain credentials for the subscription.",
				Validators:          []validator.String{guid("subscription_id")},
				PlanModifiers:       replace,
			},
			"region": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Azure region the Event Hub stack is deployed in.",
				PlanModifiers:       replace,
			},
			"resource_group": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Resource group holding the Event Hub namespaces.",
			},
			"event_hub_namespaces": schema.ListNestedAttribute{
				Required: true,
				MarkdownDescription: "Event Hub namespaces provisioned for this subscription, each with the hubs it holds. " +
					"A hub's vendor is not recorded separately; it is carried by the hub name.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Event Hub namespace name.",
						},
						"event_hubs": schema.ListNestedAttribute{
							Required:            true,
							MarkdownDescription: "Event hubs within this namespace.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "Event hub name. The vendor is derived from it.",
									},
									"consumer_groups": schema.ListAttribute{
										Required:            true,
										ElementType:         types.StringType,
										MarkdownDescription: "Consumer groups on this hub that Trend Vision One may read from.",
									},
								},
							},
						},
					},
				},
			},
			"scm_device_token": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Device token issued to this resource's caller. Cloud Log Monitoring authenticates registration and teardown calls with this token instead of the provider's own api_key, since it is scoped to this one subscription.",
			},
		},
	}
}

func (r *udcEventHubInfoResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *trendmicro.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = &api.ClmClient{Client: client}
	tflog.Debug(ctx, "[CLM UDC EventHub Info] resource configured successfully")
}

func (r *udcEventHubInfoResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan udcEventHubInfoModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("[CLM UDC EventHub Info][Create] registering %d namespace(s) in %s/%s",
		len(plan.EventHubNamespaces), plan.SubscriptionID.ValueString(), plan.Region.ValueString()))

	// Upsert rather than create: a customer who deletes the resource group and redeploys loses
	// the Azure resources and the Terraform state together while the backend record survives.
	// Adopting that record keeps the redeploy clean instead of failing on a duplicate.
	if err := r.client.UpsertUdcEventHubInfo(plan.DeviceToken.ValueString(), plan.toAPI()); err != nil {
		resp.Diagnostics.AddError(
			"[CLM UDC EventHub Info][Create] Error Registering Event Hub Info",
			fmt.Sprintf("Failed to register Event Hub stack: %s", err),
		)

		return
	}

	plan.ID = plan.computeID()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read is a no-op: the backend exposes no API that returns a registered stack's state to the
// device token that registered it, so there is nothing to refresh state against. The resource's
// own state stays authoritative between applies.
func (r *udcEventHubInfoResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state udcEventHubInfoModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *udcEventHubInfoResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan udcEventHubInfoModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There is no separate update endpoint: enabling a vendor, adding a consumer group and
	// repacking namespaces all arrive here as the same upsert Create uses, since none of them
	// changes the tenant, subscription or region the record is keyed on.
	if err := r.client.UpsertUdcEventHubInfo(plan.DeviceToken.ValueString(), plan.toAPI()); err != nil {
		resp.Diagnostics.AddError(
			"[CLM UDC EventHub Info][Update] Error Updating Event Hub Info",
			fmt.Sprintf("Failed to update Event Hub stack: %s", err),
		)

		return
	}

	plan.ID = plan.computeID()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *udcEventHubInfoResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state udcEventHubInfoModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteUdcEventHubInfo(state.DeviceToken.ValueString(), state.SubscriptionID.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"[CLM UDC EventHub Info][Delete] Error Deleting Event Hub Info",
			fmt.Sprintf("Failed to delete Event Hub stack: %s", err),
		)
	}
}
