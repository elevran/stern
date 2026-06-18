package config

import (
	"context"
	"fmt"
	"io"
	"strings"

	gh "github.com/google/go-github/v72/github"

	"github.com/elevran/stern/internal/ghclient"
)

type labelUpdateItem struct {
	desired LabelDefinition
	current *gh.Label
}

// LabelPlan groups label changes by kind after a DiffLabels call.
type LabelPlan struct {
	Creates   []LabelDefinition // present in config, absent in repo
	Updates   []labelUpdateItem // present in both, but color or description differs
	Unchanged []string          // present in both, identical
	Extras    []*gh.Label       // present in repo, absent in config
}

// DiffLabels computes the reconciliation plan between desired and current repo labels.
func DiffLabels(desired []LabelDefinition, current []*gh.Label) LabelPlan {
	index := make(map[string]*gh.Label, len(current))
	for _, l := range current {
		index[strings.ToLower(l.GetName())] = l
	}

	var plan LabelPlan
	seen := make(map[string]bool)

	for i := range desired {
		d := &desired[i]
		key := strings.ToLower(d.Name)
		seen[key] = true

		cur, exists := index[key]
		if !exists {
			plan.Creates = append(plan.Creates, *d)
			continue
		}
		if !strings.EqualFold(cur.GetColor(), d.Color) || cur.GetDescription() != d.Description {
			plan.Updates = append(plan.Updates, labelUpdateItem{desired: *d, current: cur})
		} else {
			plan.Unchanged = append(plan.Unchanged, d.Name)
		}
	}

	for _, l := range current {
		if !seen[strings.ToLower(l.GetName())] {
			plan.Extras = append(plan.Extras, l)
		}
	}
	return plan
}

// Print writes a human-readable plan to w.
func (p *LabelPlan) Print(w io.Writer) {
	for _, d := range p.Creates {
		_, _ = fmt.Fprintf(w, "  CREATE  %s  #%s  %q\n", d.Name, d.Color, d.Description)
	}
	for _, u := range p.Updates {
		_, _ = fmt.Fprintf(w, "  UPDATE  %s\n", u.desired.Name)
	}
	for _, name := range p.Unchanged {
		_, _ = fmt.Fprintf(w, "  OK      %s\n", name)
	}
	for _, l := range p.Extras {
		_, _ = fmt.Fprintf(w, "  EXTRA   %s\n", l.GetName())
	}
}

// Apply executes CREATE and UPDATE operations.
// EXTRA labels are deleted only when prune is true.
func (p *LabelPlan) Apply(ctx context.Context, ghc ghclient.Client, owner, repo string, prune bool) error {
	for _, d := range p.Creates {
		label := &gh.Label{
			Name:        gh.Ptr(d.Name),
			Color:       gh.Ptr(d.Color),
			Description: gh.Ptr(d.Description),
		}
		if err := ghc.CreateLabel(ctx, owner, repo, label); err != nil {
			return fmt.Errorf("creating label %q: %w", d.Name, err)
		}
	}
	for _, u := range p.Updates {
		label := &gh.Label{
			Name:        gh.Ptr(u.desired.Name),
			Color:       gh.Ptr(u.desired.Color),
			Description: gh.Ptr(u.desired.Description),
		}
		if err := ghc.UpdateLabel(ctx, owner, repo, u.current.GetName(), label); err != nil {
			return fmt.Errorf("updating label %q: %w", u.desired.Name, err)
		}
	}
	if prune {
		for _, l := range p.Extras {
			if err := ghc.DeleteLabel(ctx, owner, repo, l.GetName()); err != nil {
				return fmt.Errorf("deleting label %q: %w", l.GetName(), err)
			}
		}
	}
	return nil
}
