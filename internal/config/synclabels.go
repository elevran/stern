package config

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/elevran/stern/internal/github"
)

type labelUpdateItem struct {
	desired LabelDefinition
	current github.Label
}

// LabelPlan groups label changes by kind after a DiffLabels call.
type LabelPlan struct {
	Creates   []LabelDefinition // present in config, absent in repo
	Updates   []labelUpdateItem // present in both, but color or description differs
	Unchanged []string          // present in both, identical
	Extras    []github.Label    // present in repo, absent in config
}

// DiffLabels computes the reconciliation plan between desired and current repo labels.
func DiffLabels(desired []LabelDefinition, current []github.Label) LabelPlan {
	index := make(map[string]github.Label, len(current))
	for _, l := range current {
		index[strings.ToLower(l.Name)] = l
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
		if !strings.EqualFold(cur.Color, d.Color) || cur.Description != d.Description {
			plan.Updates = append(plan.Updates, labelUpdateItem{desired: *d, current: cur})
		} else {
			plan.Unchanged = append(plan.Unchanged, d.Name)
		}
	}

	for _, l := range current {
		if !seen[strings.ToLower(l.Name)] {
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
		_, _ = fmt.Fprintf(w, "  EXTRA   %s\n", l.Name)
	}
}

// Apply executes CREATE and UPDATE operations.
// EXTRA labels are deleted only when prune is true.
func (p *LabelPlan) Apply(ctx context.Context, ghc github.Client, owner, repo string, prune bool) error {
	for _, d := range p.Creates {
		label := github.Label{
			Name:        d.Name,
			Color:       d.Color,
			Description: d.Description,
		}
		if err := ghc.CreateLabel(ctx, owner, repo, label); err != nil {
			return fmt.Errorf("creating label %q: %w", d.Name, err)
		}
	}
	for _, u := range p.Updates {
		label := github.Label{
			Name:        u.desired.Name,
			Color:       u.desired.Color,
			Description: u.desired.Description,
		}
		if err := ghc.UpdateLabel(ctx, owner, repo, u.current.Name, label); err != nil {
			return fmt.Errorf("updating label %q: %w", u.desired.Name, err)
		}
	}
	if prune {
		for _, l := range p.Extras {
			if err := ghc.DeleteLabel(ctx, owner, repo, l.Name); err != nil {
				return fmt.Errorf("deleting label %q: %w", l.Name, err)
			}
		}
	}
	return nil
}
