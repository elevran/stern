package config_test

import (
	"strings"
	"testing"

	"github.com/elevran/stern/internal/config"
)

// defaultSizeBuckets mirrors config.defaultSizeBuckets so tests can exercise
// the default-bucket shape without invoking the unexported applyDefaults().
func defaultSizeBuckets() []config.SizeBucket {
	return []config.SizeBucket{
		{Name: "XS", Max: 10},
		{Name: "S", Min: 11, Max: 30},
		{Name: "M", Min: 31, Max: 100},
		{Name: "L", Min: 101, Max: 300},
		{Name: "XL", Min: 301, Max: 1000},
		{Name: "XXL", Min: 1001},
	}
}

func TestSizeValidate_DefaultsClean(t *testing.T) {
	o := &config.Options{
		Merge: config.MergeOptions{Strategy: "native", Method: "squash", BlockingLabels: []string{"x"}},
		Size:  config.SizeOptions{Buckets: defaultSizeBuckets()},
	}
	errs := o.Validate()
	for _, e := range errs {
		if strings.Contains(e.Error(), "size.buckets") && strings.Contains(e.Error(), "ERROR") {
			t.Errorf("unexpected ERROR for default size buckets: %v", e)
		}
	}
}

func TestSizeValidate_EmptyName(t *testing.T) {
	o := &config.Options{
		Merge: config.MergeOptions{Strategy: "native", Method: "squash", BlockingLabels: []string{"x"}},
		Size: config.SizeOptions{
			Buckets: []config.SizeBucket{
				{Name: "OK", Max: 5},
				{Name: "", Min: 6, Max: 10},
			},
		},
	}
	errs := o.Validate()
	hasErr := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "size.buckets[1].name") && strings.Contains(e.Error(), "ERROR") {
			hasErr = true
		}
	}
	if !hasErr {
		t.Errorf("expected ERROR for empty bucket name, got %v", errs)
	}
}

func TestSizeValidate_OverlappingRanges(t *testing.T) {
	o := &config.Options{
		Merge: config.MergeOptions{Strategy: "native", Method: "squash", BlockingLabels: []string{"x"}},
		Size: config.SizeOptions{
			Buckets: []config.SizeBucket{
				{Name: "A", Min: 1, Max: 10},
				{Name: "B", Min: 5, Max: 15},
			},
		},
	}
	errs := o.Validate()
	hasErr := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "overlap") && strings.Contains(e.Error(), "ERROR") {
			hasErr = true
		}
	}
	if !hasErr {
		t.Errorf("expected ERROR for overlapping ranges, got %v", errs)
	}
}

func TestSizeValidate_NonOverlappingRanges(t *testing.T) {
	o := &config.Options{
		Merge: config.MergeOptions{Strategy: "native", Method: "squash", BlockingLabels: []string{"x"}},
		Size: config.SizeOptions{
			Buckets: []config.SizeBucket{
				{Name: "A", Min: 1, Max: 10},
				{Name: "B", Min: 11, Max: 20},
			},
		},
	}
	errs := o.Validate()
	for _, e := range errs {
		if strings.Contains(e.Error(), "size.buckets") && strings.Contains(e.Error(), "ERROR") {
			t.Errorf("unexpected ERROR: %v", e)
		}
	}
}

func TestSizeValidate_OpenEndedNoOverlap(t *testing.T) {
	o := &config.Options{
		Merge: config.MergeOptions{Strategy: "native", Method: "squash", BlockingLabels: []string{"x"}},
		Size: config.SizeOptions{
			Buckets: []config.SizeBucket{
				{Name: "LOW", Max: 10},
				{Name: "HIGH", Min: 11},
			},
		},
	}
	errs := o.Validate()
	for _, e := range errs {
		if strings.Contains(e.Error(), "size.buckets") && strings.Contains(e.Error(), "ERROR") {
			t.Errorf("unexpected ERROR for open-ended buckets: %v", e)
		}
	}
}

func TestSizeValidate_EnabledButEmpty(t *testing.T) {
	o := &config.Options{
		Plugins: []string{"size"},
		Merge:   config.MergeOptions{Strategy: "native", Method: "squash", BlockingLabels: []string{"x"}},
		// No explicit size config — defaults would normally be applied by
		// LoadFromFile, but here we test Validate() directly without defaults,
		// so Buckets is nil.
	}
	errs := o.Validate()
	hasWarn := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "size.buckets") && strings.Contains(e.Error(), "WARN") {
			hasWarn = true
		}
	}
	if !hasWarn {
		t.Errorf("expected WARN when size enabled with no buckets, got %v", errs)
	}
}

func TestSizeValidate_NegativeValues(t *testing.T) {
	o := &config.Options{
		Merge: config.MergeOptions{Strategy: "native", Method: "squash", BlockingLabels: []string{"x"}},
		Size: config.SizeOptions{
			Buckets: []config.SizeBucket{
				{Name: "BAD", Min: -1, Max: 5},
			},
		},
	}
	errs := o.Validate()
	hasErr := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "non-negative") && strings.Contains(e.Error(), "ERROR") {
			hasErr = true
		}
	}
	if !hasErr {
		t.Errorf("expected ERROR for negative min/max, got %v", errs)
	}
}
