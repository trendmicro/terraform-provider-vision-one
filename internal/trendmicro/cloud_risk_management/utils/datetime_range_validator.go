package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = &datetimeRangeValidator{}

type datetimeRangeValidator struct {
	rangeStart int
	rangeEnd   int
	unit       time.Duration
}

func (v datetimeRangeValidator) Description(_ context.Context) string {
	unitName := durationUnitDisplayName(v.unit, v.rangeEnd)
	return fmt.Sprintf("value must be between %d and %d %s from now", v.rangeStart, v.rangeEnd, unitName)
}

func (v datetimeRangeValidator) MarkdownDescription(_ context.Context) string {
	unitName := durationUnitDisplayName(v.unit, v.rangeEnd)
	return fmt.Sprintf("value must be between %d and %d %s from now", v.rangeStart, v.rangeEnd, unitName)
}

func (v datetimeRangeValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if value == "" {
		// Empty value is allowed (treated as "not set"), so skip validation.
		return
	}

	disabledUntil, err := time.Parse(time.RFC3339, value)
	if err != nil {
		// Let ISO8601Datetime validator report datetime format issues.
		return
	}

	if v.unit <= 0 || v.rangeStart < 0 || v.rangeEnd < v.rangeStart {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Datetime Range Validator Configuration",
			fmt.Sprintf("Validator configuration must satisfy: unit > 0, rangeStart >= 0, and rangeEnd >= rangeStart. Got rangeStart=%d, rangeEnd=%d, unit=%s.", v.rangeStart, v.rangeEnd, v.unit),
		)
		return
	}

	now := time.Now().UTC()
	delta := disabledUntil.Sub(now)
	minDelta := time.Duration(v.rangeStart) * v.unit
	maxDelta := time.Duration(v.rangeEnd) * v.unit

	if delta < minDelta || delta > maxDelta {
		unitName := durationUnitDisplayName(v.unit, v.rangeEnd)
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Datetime Range",
			fmt.Sprintf("The field %q must be between %d and %d %s from the current time.", req.Path.String(), v.rangeStart, v.rangeEnd, unitName),
		)
	}
}

// durationUnitDisplayName returns a human-readable name for the duration unit with proper pluralization.
// Supports units greater than or equal to seconds (second, minute, hour).
func durationUnitDisplayName(unit time.Duration, count int) string {
	var name string
	switch unit {
	case time.Hour:
		name = "hour"
	case time.Minute:
		name = "minute"
	case time.Second:
		name = "second"
	default:
		return unit.String()
	}
	if count != 1 {
		name += "s"
	}
	return name
}

// DatetimeRangeFromNow validates an RFC3339 datetime to be within [rangeStart, rangeEnd] units from now.
func DatetimeRangeFromNow(rangeStart, rangeEnd int, datetimeUnit time.Duration) validator.String {
	return datetimeRangeValidator{
		rangeStart: rangeStart,
		rangeEnd:   rangeEnd,
		unit:       datetimeUnit,
	}
}
