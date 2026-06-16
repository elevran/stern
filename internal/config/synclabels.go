package config

import (
	"context"
	"fmt"
	"io"
	"strings"

	gh "github.com/google/go-github/v72/github"

	ghclient "github.com/elevran/stern/internal/github"
)

// LabelAction classifies what needs to happen to a label.
type LabelAction int

const (
	LabelOK     LabelAction = iota // exists with correct color and description
	LabelCreate                    // in config, not in repo
	LabelUpdate                    // in repo, but color or description differs
	LabelExtra                     // in repo, not in config
)

// LabelDiff describes a single label reconciliation action.
type LabelDiff struct {
	Name    string
	Action  LabelAction
	Current *gh.Label       // nil for Create
	Desired *LabelDefinition // nil for Extra
}

// DiffLabels computes the reconciliation plan between label_definitions and
// current repo labels.
func DiffLabels(desired []LabelDefinition, current []*gh.Label) []LabelDiff {
	// Index current labels by lowercased name.
	index := make(map[string]*gh.Label, len(current))
	for _, l := range current {
		index[strings.ToLower(l.GetName())] = l
	}

	seen := make(map[string]bool)
	var diffs []LabelDiff

	for i := range desired {
		d := &desired[i]
		key := strings.ToLower(d.Name)
		seen[key] = true

		cur, exists := index[key]
		if !exists {
			diffs = append(diffs, LabelDiff{Name: d.Name, Action: LabelCreate, Desired: d})
			continue
		}
		if !strings.EqualFold(cur.GetColor(), d.Color) || cur.GetDescription() != d.Description {
			diffs = append(diffs, LabelDiff{Name: d.Name, Action: LabelUpdate, Current: cur, Desired: d})
		} else {
			diffs = append(diffs, LabelDiff{Name: d.Name, Action: LabelOK, Current: cur, Desired: d})
		}
	}

	for _, l := range current {
		if !seen[strings.ToLower(l.GetName())] {
			diffs = append(diffs, LabelDiff{Name: l.GetName(), Action: LabelExtra, Current: l})
		}
	}
	return diffs
}

// PrintLabelPlan writes a human-readable diff table to w.
func PrintLabelPlan(w io.Writer, diffs []LabelDiff) {
	for _, d := range diffs {
		switch d.Action {
		case LabelCreate:
			fmt.Fprintf(w, "  CREATE  %s  #%s  %q\n", d.Name, d.Desired.Color, d.Desired.Description)
		case LabelUpdate:
			fmt.Fprintf(w, "  UPDATE  %s\n", d.Name)
		case LabelOK:
			fmt.Fprintf(w, "  OK      %s\n", d.Name)
		case LabelExtra:
			fmt.Fprintf(w, "  EXTRA   %s\n", d.Name)
		}
	}
}

// ApplyLabelDiffs applies the CREATE and UPDATE diffs to the repo.
// EXTRA labels are only deleted when prune is true.
func ApplyLabelDiffs(ctx context.Context, ghc ghclient.Client, owner, repo string, diffs []LabelDiff, prune bool) error {
	for _, d := range diffs {
		switch d.Action {
		case LabelCreate:
			label := &gh.Label{
				Name:        gh.Ptr(d.Desired.Name),
				Color:       gh.Ptr(d.Desired.Color),
				Description: gh.Ptr(d.Desired.Description),
			}
			if err := ghc.CreateLabel(ctx, owner, repo, label); err != nil {
				return fmt.Errorf("creating label %q: %w", d.Name, err)
			}
		case LabelUpdate:
			label := &gh.Label{
				Name:        gh.Ptr(d.Desired.Name),
				Color:       gh.Ptr(d.Desired.Color),
				Description: gh.Ptr(d.Desired.Description),
			}
			if err := ghc.UpdateLabel(ctx, owner, repo, d.Current.GetName(), label); err != nil {
				return fmt.Errorf("updating label %q: %w", d.Name, err)
			}
		case LabelExtra:
			if prune {
				if err := ghc.DeleteLabel(ctx, owner, repo, d.Name); err != nil {
					return fmt.Errorf("deleting label %q: %w", d.Name, err)
				}
			}
		}
	}
	return nil
}
