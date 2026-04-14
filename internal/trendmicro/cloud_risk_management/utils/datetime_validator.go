package utils

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = &datetimeValidator{}

type datetimeValidator struct{}

func (v datetimeValidator) Description(ctx context.Context) string {
	return "value must be a valid ISO 8601 datetime string (RFC3339 format)"
}

func (v datetimeValidator) MarkdownDescription(ctx context.Context) string {
	return "value must be a valid ISO 8601 datetime string (RFC3339 format)"
}

func (v datetimeValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// If the value is null or unknown, skip validation
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	valueString := req.ConfigValue.ValueString()

	// If empty string, skip validation (it's allowed as "not set")
	if valueString == "" {
		return
	}

	// Try parsing as RFC3339 (ISO 8601)
	_, err := time.Parse(time.RFC3339, valueString)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid ISO 8601 Datetime Format",
			"The value must be a valid ISO 8601 datetime string in RFC3339 format (e.g., '2026-12-31T23:59:59Z' or '2026-12-31T23:59:59+00:00'). "+
				"Error: "+err.Error(),
		)
		return
	}
}

// ISO8601Datetime returns a new datetime validator that validates ISO 8601 datetime strings
func ISO8601Datetime() validator.String {
	return datetimeValidator{}
}
