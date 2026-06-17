package data_sources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"terraform-provider-vision-one/internal/trendmicro/data_security_posture_management/gcp/data-sources/config"
	resourcesconfig "terraform-provider-vision-one/internal/trendmicro/data_security_posture_management/gcp/resources/config"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/oauth2/google"
	storagev1 "google.golang.org/api/storage/v1"
	"google.golang.org/api/option"
)

var (
	_ datasource.DataSource = &LegacyStateRegionsDataSource{}
)

type LegacyStateRegionsDataSource struct{}

type legacyStateRegionsModel struct {
	ID                types.String `tfsdk:"id"`
	ProjectID         types.String `tfsdk:"project_id"`
	ServiceAccountKey types.String `tfsdk:"service_account_key"`
	BucketName        types.String `tfsdk:"bucket_name"`
	Regions           types.Set    `tfsdk:"regions"`
}

func NewLegacyStateRegionsDataSource() datasource.DataSource {
	return &LegacyStateRegionsDataSource{}
}

func (d *LegacyStateRegionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + config.DATA_SOURCE_TYPE_DSPM_LEGACY_STATE_REGIONS
}

func (d *LegacyStateRegionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Discovers the GCP regions a legacy Terraform Package Solution deployment used, by reading `gs://trendmicro-v1-{project_id}/default.tfstate`. Returns an empty set when no legacy bucket or state file exists. Pair with `visionone_dspm_legacy_cleanup_region` to drive per-region cleanup before redeploying via the TFP path.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Equals `project_id`.",
				Computed:            true,
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The GCP project ID whose legacy state bucket should be inspected.",
				Required:            true,
			},
			"service_account_key": schema.StringAttribute{
				MarkdownDescription: "Base64-encoded JSON service account key used to authenticate with GCS for reading the legacy state file. Optional — three common patterns:\n" +
					"\n" +
					"- **CAM-integrated** (recommended): set to `visionone_cam_service_account_integration.comprehensive.private_key`.\n" +
					"- **BYO key**: set to a base64-encoded JSON key for any service account with `storage.objects.get` on the legacy bucket.\n" +
					"- **ADC**: omit the attribute entirely. Falls back to Application Default Credentials.",
				Optional:  true,
				Sensitive: true,
			},
			"bucket_name": schema.StringAttribute{
				MarkdownDescription: "The legacy state bucket name that was probed (`trendmicro-v1-{project_id}`).",
				Computed:            true,
			},
			"regions": schema.SetAttribute{
				MarkdownDescription: "Set of GCP region names extracted from the legacy state file. Empty when no legacy state exists.",
				ElementType:         types.StringType,
				Computed:            true,
			},
		},
	}
}

func (d *LegacyStateRegionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data legacyStateRegionsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := data.ProjectID.ValueString()
	bucketName := resourcesconfig.LEGACY_GCP_GCS_BUCKET_PREFIX + projectID
	data.ID = types.StringValue(projectID)
	data.BucketName = types.StringValue(bucketName)

	clientOptions, err := buildStorageClientOptions(ctx, data.ServiceAccountKey.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("[Legacy State Regions] Invalid service account key", err.Error())
		return
	}

	regions, err := discoverRegionsFromLegacyState(ctx, bucketName, clientOptions)
	if err != nil {
		resp.Diagnostics.AddError("[Legacy State Regions] Failed to read legacy state", err.Error())
		return
	}

	tflog.Info(ctx, fmt.Sprintf("[Legacy State Regions] project=%s bucket=%s found=%d regions", projectID, bucketName, len(regions)))

	setVal, diag := types.SetValueFrom(ctx, types.StringType, regions)
	resp.Diagnostics.Append(diag...)
	data.Regions = setVal

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func buildStorageClientOptions(ctx context.Context, encodedKey string) ([]option.ClientOption, error) {
	if encodedKey == "" {
		return nil, nil
	}
	keyJSON, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("decode service account key: %w", err)
	}
	creds, err := google.CredentialsFromJSON(ctx, keyJSON, storagev1.DevstorageReadOnlyScope)
	if err != nil {
		return nil, fmt.Errorf("credentials from service account key: %w", err)
	}
	return []option.ClientOption{option.WithCredentials(creds)}, nil
}

// regionTokenRE matches `["<token>"]` in tfstate module addresses; results are
// filtered to last-char-digit since GCP region names always end with a digit.
var regionTokenRE = regexp.MustCompile(`\["([^"]+)"\]`)

// discoverRegionsFromLegacyState returns regions from default.tfstate, or
// empty slice (no error) when bucket/object is missing — the "no legacy state" signal.
func discoverRegionsFromLegacyState(ctx context.Context, bucketName string, clientOptions []option.ClientOption) ([]string, error) {
	svc, err := storagev1.NewService(ctx, clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("storage client: %w", err)
	}

	rc, err := svc.Objects.Get(bucketName, resourcesconfig.LEGACY_GCP_STATE_FILE_NAME).Context(ctx).Download()
	if err != nil {
		if isStorageNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("download legacy state: %w", err)
	}
	defer rc.Body.Close()

	body, err := io.ReadAll(rc.Body)
	if err != nil {
		return nil, fmt.Errorf("read legacy state: %w", err)
	}

	var state struct {
		Resources []struct {
			Module string `json:"module"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, fmt.Errorf("parse legacy state: %w", err)
	}

	seen := make(map[string]struct{})
	for _, r := range state.Resources {
		for _, m := range regionTokenRE.FindAllStringSubmatch(r.Module, -1) {
			tok := m[1]
			if tok == "" {
				continue
			}
			last := tok[len(tok)-1]
			if last < '0' || last > '9' {
				continue
			}
			seen[tok] = struct{}{}
		}
	}

	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	return out, nil
}

func isStorageNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// GCS returns 403 instead of 404 when the caller lacks permission — it deliberately
	// does not reveal bucket existence. Treat as "no legacy state" (same as missing bucket).
	return strings.Contains(msg, "403") || strings.Contains(msg, "404") ||
		strings.Contains(msg, "notFound") || strings.Contains(msg, "doesn't exist")
}
