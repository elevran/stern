package owners

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/elevran/stern/internal/github"
)

// File represents a parsed OWNERS file.
type File struct {
	Approvers []string `yaml:"approvers"`
	Reviewers []string `yaml:"reviewers"`
}

// Aliases represents the OWNERS_ALIASES file at the root.
type Aliases struct {
	Aliases map[string][]string `yaml:"aliases"`
}

// ResolvedOwners holds the expanded approvers and reviewers for a set of paths.
type ResolvedOwners struct {
	Approvers []string
	Reviewers []string
}

// LoadForPaths fetches and merges OWNERS files for changedPaths from the
// repository at the given ref. It walks from each file's directory up to the
// root, collecting approvers and reviewers. Duplicate logins are deduplicated.
// If no OWNERS files are found, it returns empty slices (caller should treat
// this as "any org member may approve/review").
//
// A non-404 error from the ContentClient (network failure, rate-limit, etc.)
// is returned to the caller — fail-closed rather than treating the transient
// error as "no OWNERS here, keep walking up", which would silently widen
// approval authority.
func LoadForPaths(ctx context.Context, ghc github.ContentClient, owner, repo, ref string, changedPaths []string) (*ResolvedOwners, error) {
	aliases, err := loadAliases(ctx, ghc, owner, repo, ref)
	if err != nil {
		return nil, err
	}

	approverSet := make(map[string]bool)
	reviewerSet := make(map[string]bool)

	for _, path := range changedPaths {
		clean := filepath.Clean(path)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			continue
		}
		dirs := ancestorDirs(clean)
		for _, dir := range dirs {
			ownersPath := ownersFilePath(dir)
			data, err := ghc.GetFileContent(ctx, owner, repo, ownersPath, ref)
			if err != nil {
				if !github.IsNotFoundError(err) {
					return nil, fmt.Errorf("loading %s@%s: %w", ownersPath, ref, err)
				}
				continue // 404: no OWNERS in this directory
			}
			var f File
			if err := yaml.Unmarshal(data, &f); err != nil {
				logrus.WithError(err).WithField("path", ownersPath).Warn("OWNERS file exists but could not be parsed; owners for this directory will be skipped")
				continue
			}
			for _, a := range f.Approvers {
				for _, login := range expandAlias(a, aliases) {
					approverSet[strings.ToLower(login)] = true
				}
			}
			for _, r := range f.Reviewers {
				for _, login := range expandAlias(r, aliases) {
					reviewerSet[strings.ToLower(login)] = true
				}
			}
		}
	}

	return &ResolvedOwners{
		Approvers: slices.Sorted(maps.Keys(approverSet)),
		Reviewers: slices.Sorted(maps.Keys(reviewerSet)),
	}, nil
}

// IsApprover reports whether login is in the approvers set.
func (r *ResolvedOwners) IsApprover(login string) bool {
	for _, a := range r.Approvers {
		if strings.EqualFold(a, login) {
			return true
		}
	}
	return false
}

// IsReviewer reports whether login is in the reviewers set.
func (r *ResolvedOwners) IsReviewer(login string) bool {
	for _, rv := range r.Reviewers {
		if strings.EqualFold(rv, login) {
			return true
		}
	}
	return false
}

// HasOwners reports whether any OWNERS files were found (non-empty result).
func (r *ResolvedOwners) HasOwners() bool {
	return len(r.Approvers) > 0 || len(r.Reviewers) > 0
}

// UncoveredFiles returns the subset of changedPaths that have an OWNERS chain
// but no approver matching login. Paths with no OWNERS file are considered
// covered (anyone may approve them).
//
// A non-404 error from the ContentClient is returned to the caller — same
// fail-closed rationale as LoadForPaths.
func UncoveredFiles(
	ctx context.Context,
	ghc github.ContentClient,
	owner, repo, ref, login string,
	changedPaths []string,
) ([]string, error) {
	aliases, err := loadAliases(ctx, ghc, owner, repo, ref)
	if err != nil {
		return nil, err
	}
	var uncovered []string
	for _, path := range changedPaths {
		clean := filepath.Clean(path)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			continue
		}
		covered, err := coveredByLogin(ctx, ghc, owner, repo, ref, clean, login, aliases)
		if err != nil {
			return nil, err
		}
		if !covered {
			uncovered = append(uncovered, clean)
		}
	}
	return uncovered, nil
}

// coveredByLogin walks the OWNERS chain for path from most-specific directory
// to root. Returns true if login is an approver in any ancestor OWNERS, or if
// no OWNERS chain exists anywhere (open directory).
//
// Non-404 errors abort the walk and surface to the caller — treating a
// transient API failure as "no OWNERS here" would silently widen authority.
func coveredByLogin(ctx context.Context, ghc github.ContentClient, owner, repo, ref, path, login string, aliases *Aliases) (bool, error) {
	for _, dir := range ancestorDirs(path) {
		data, err := ghc.GetFileContent(ctx, owner, repo, ownersFilePath(dir), ref)
		if err != nil {
			if !github.IsNotFoundError(err) {
				return false, fmt.Errorf("loading %s@%s: %w", ownersFilePath(dir), ref, err)
			}
			continue // 404: no OWNERS in this directory, keep walking up
		}
		var f File
		if err := yaml.Unmarshal(data, &f); err != nil {
			continue
		}
		if len(f.Approvers) == 0 {
			continue // OWNERS exists but lists no approvers; keep walking up
		}
		// This level has explicit approvers — authoritative for this path.
		for _, a := range f.Approvers {
			for _, expanded := range expandAlias(a, aliases) {
				if strings.EqualFold(expanded, login) {
					return true, nil
				}
			}
		}
		return false, nil // level has approvers but login is not among them
	}
	return true, nil // no OWNERS found anywhere — open directory
}

// ancestorDirs returns the directory path and all its ancestors up to root,
// ordered from most specific (deepest) to least specific (root ".").
func ancestorDirs(filePath string) []string {
	dir := filepath.Dir(filePath)
	var dirs []string
	for {
		dirs = append(dirs, dir)
		if dir == "." || dir == "/" || dir == "" {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

// ownersFilePath returns the OWNERS file path for a given directory.
func ownersFilePath(dir string) string {
	if dir == "." || dir == "" {
		return "OWNERS"
	}
	return dir + "/OWNERS"
}

// loadAliases fetches and parses the root OWNERS_ALIASES file.
// Returns an empty Aliases and no error if the file doesn't exist.
// A non-404 error is returned to the caller so transient API failures
// don't silently disable alias expansion.
func loadAliases(ctx context.Context, ghc github.ContentClient, owner, repo, ref string) (*Aliases, error) {
	data, err := ghc.GetFileContent(ctx, owner, repo, "OWNERS_ALIASES", ref)
	if err != nil {
		if github.IsNotFoundError(err) {
			return &Aliases{Aliases: make(map[string][]string)}, nil
		}
		return nil, fmt.Errorf("loading OWNERS_ALIASES@%s: %w", ref, err)
	}
	var a Aliases
	if err := yaml.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parsing OWNERS_ALIASES@%s: %w", ref, err)
	}
	if a.Aliases == nil {
		a.Aliases = make(map[string][]string)
	}
	return &a, nil
}

// expandAlias expands a name to a list of logins. If name matches an alias
// key, the alias expansion is returned. Otherwise, the name itself is returned.
func expandAlias(name string, aliases *Aliases) []string {
	if aliases == nil {
		return []string{name}
	}
	if members, ok := aliases.Aliases[name]; ok {
		return members
	}
	return []string{name}
}
