package config

import (
	"fmt"
	"math"
)

// SizeOptions configures the automatic size/* label plugin.
type SizeOptions struct {
	Buckets []SizeBucket `yaml:"buckets"`
}

// SizeBucket maps a range of changed lines to a bucket name.
// Min == 0 means no lower bound; Max == 0 means no upper bound.
type SizeBucket struct {
	Name string `yaml:"name"`
	Min  int    `yaml:"min"`
	Max  int    `yaml:"max"`
}

func defaultSizeBuckets() []SizeBucket {
	return []SizeBucket{
		{Name: "XS", Max: 10},
		{Name: "S", Min: 11, Max: 30},
		{Name: "M", Min: 31, Max: 100},
		{Name: "L", Min: 101, Max: 300},
		{Name: "XL", Min: 301, Max: 1000},
		{Name: "XXL", Min: 1001},
	}
}

// applyDefaults fills Buckets with the defaults when unset.
func (o *SizeOptions) applyDefaults() {
	if len(o.Buckets) == 0 {
		o.Buckets = defaultSizeBuckets()
	}
}

// validate checks bucket names are non-empty and ranges don't overlap when both
// Min and Max are set. pluginEnabled is true when the "size" plugin is listed
// in options.Plugins; when enabled with no buckets (defaults not applied), a
// WARN is emitted.
func (o *SizeOptions) validate(pluginEnabled bool) []ValidationIssue {
	var issues []ValidationIssue
	if pluginEnabled && len(o.Buckets) == 0 {
		issues = append(issues, ValidationIssue{
			Level:   "WARN",
			Field:   "size.buckets",
			Message: "size plugin is enabled but buckets list is empty",
		})
		return issues
	}
	for i, b := range o.Buckets {
		if b.Name == "" {
			issues = append(issues, ValidationIssue{
				Level:   "ERROR",
				Field:   fieldPath("size.buckets", i, "name"),
				Message: "bucket name is empty",
			})
		}
		if b.Min < 0 || b.Max < 0 {
			issues = append(issues, ValidationIssue{
				Level:   "ERROR",
				Field:   fieldPath("size.buckets", i, ""),
				Message: "bucket min/max must be non-negative",
			})
		}
		if b.Min > 0 && b.Max > 0 && b.Min > b.Max {
			issues = append(issues, ValidationIssue{
				Level:   "ERROR",
				Field:   fieldPath("size.buckets", i, ""),
				Message: fmt.Sprintf("bucket %q: min (%d) > max (%d)", b.Name, b.Min, b.Max),
			})
		}
	}
	// Overlap check. Unset Max means unbounded — treat as math.MaxInt so two
	// open-lower-bound buckets (e.g. two `{Max:N}` entries) don't silently
	// overlap.
	for i := range o.Buckets {
		a := o.Buckets[i]
		aMax := a.Max
		if aMax == 0 {
			aMax = math.MaxInt
		}
		for j := i + 1; j < len(o.Buckets); j++ {
			b := o.Buckets[j]
			bMax := b.Max
			if bMax == 0 {
				bMax = math.MaxInt
			}
			if a.Min <= bMax && b.Min <= aMax {
				issues = append(issues, ValidationIssue{
					Level: "ERROR",
					Field: fieldPath("size.buckets", i, ""),
					Message: fmt.Sprintf(
						"bucket %q (min=%d, max=%d) overlaps with bucket %q (min=%d, max=%d)",
						a.Name, a.Min, a.Max, b.Name, b.Min, b.Max,
					),
				})
			}
		}
	}
	return issues
}

// fieldPath builds a "size.buckets[N].suffix" style path for validation messages.
func fieldPath(prefix string, idx int, suffix string) string {
	if suffix == "" {
		return fmt.Sprintf("%s[%d]", prefix, idx)
	}
	return fmt.Sprintf("%s[%d].%s", prefix, idx, suffix)
}
