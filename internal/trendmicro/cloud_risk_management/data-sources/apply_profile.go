package datasources

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"terraform-provider-vision-one/internal/trendmicro"
	"terraform-provider-vision-one/internal/trendmicro/cloud_risk_management/api"
	crm_dto "terraform-provider-vision-one/pkg/dto/cloud_risk_management"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &ApplyProfileDataSource{}
	_ datasource.DataSourceWithConfigure = &ApplyProfileDataSource{}
)

func NewApplyProfileDataSource() datasource.DataSource {
	return &ApplyProfileDataSource{}
}

// ApplyProfileDataSource applies a profile to accounts.
type ApplyProfileDataSource struct {
	client *api.CrmClient
}

type ApplyProfileIncludeModel struct {
	Exceptions types.Bool `tfsdk:"exceptions"`
}

type ApplyProfileDataSourceModel struct {
	ID         types.String              `tfsdk:"id"`
	ProfileID  types.String              `tfsdk:"profile_id"`
	AccountIDs []types.String            `tfsdk:"account_ids"`
	Mode       types.String              `tfsdk:"mode"`
	Notes      types.String              `tfsdk:"notes"`
	Include    *ApplyProfileIncludeModel `tfsdk:"include"`
}

func (d *ApplyProfileDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_crm_apply_profile"
}

func (d *ApplyProfileDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Applies a Cloud Risk Management profile to one or more accounts.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"profile_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the profile to apply.",
			},
			"account_ids": schema.ListAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Account IDs to apply the profile to.",
			},
			"mode": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Apply mode: fill-gaps, overwrite, or replace.",
			},
			"notes": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Notes for the apply request.",
			},
			"include": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Optional include settings. Only supported in overwrite mode.",
				Attributes: map[string]schema.Attribute{
					"exceptions": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Whether to include exceptions when applying the profile.",
					},
				},
			},
		},
	}
}

func (d *ApplyProfileDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*trendmicro.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *trendmicro.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = &api.CrmClient{Client: client}
}

func (d *ApplyProfileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ApplyProfileDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountIDs := make([]string, 0, len(data.AccountIDs))
	for _, accountID := range data.AccountIDs {
		if !accountID.IsNull() && !accountID.IsUnknown() {
			accountIDs = append(accountIDs, accountID.ValueString())
		}
	}

	request := &crm_dto.ApplyProfileRequest{
		AccountIDs: accountIDs,
		Types:      "rule",
		Mode:       data.Mode.ValueString(),
	}

	if !data.Notes.IsNull() && !data.Notes.IsUnknown() {
		request.Note = data.Notes.ValueString()
	}

	if data.Include != nil && !data.Include.Exceptions.IsNull() && !data.Include.Exceptions.IsUnknown() {
		includeExceptions := data.Include.Exceptions.ValueBool()
		request.Include = &crm_dto.ApplyProfileInclude{
			Exceptions: &includeExceptions,
		}
	}

	response, err := d.client.ApplyProfile(data.ProfileID.ValueString(), request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error applying profile",
			fmt.Sprintf("Could not apply profile %s: %s", data.ProfileID.ValueString(), err.Error()),
		)
		return
	}

	data.ID = types.StringValue(strconv.FormatInt(time.Now().Unix(), 10))

	logFields := map[string]interface{}{
		"profile_id": data.ProfileID.ValueString(),
	}
	if response.Meta.Status != "" {
		logFields["status"] = response.Meta.Status
		logFields["message"] = response.Meta.Message
	} else if len(response.Results) > 0 {
		logFields["result_count"] = len(response.Results)
	}

	tflog.Debug(ctx, "Applied profile", logFields)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}