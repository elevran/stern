package owners

import (
	"context"
	"path/filepath"
	"strings"

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
func LoadForPaths(ctx context.Context, ghc github.ContentClient, owner, repo, ref string, changedPaths []string) (*ResolvedOwners, error) {
	aliases, _ := loadAliases(ctx, ghc, owner, repo, ref)

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
				continue // no OWNERS file in this directory
			}
			var f File
			if err := yaml.Unmarshal(data, &f); err != nil {
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
		Approvers: setToSlice(approverSet),
		Reviewers: setToSlice(reviewerSet),
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
func loadAliases(ctx context.Context, ghc github.ContentClient, owner, repo, ref string) (*Aliases, error) {
	data, err := ghc.GetFileContent(ctx, owner, repo, "OWNERS_ALIASES", ref)
	if err != nil {
		return &Aliases{Aliases: make(map[string][]string)}, nil
	}
	var a Aliases
	if err := yaml.Unmarshal(data, &a); err != nil {
		return &Aliases{Aliases: make(map[string][]string)}, nil
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

func setToSlice(s map[string]bool) []string {
	result := make([]string, 0, len(s))
	for k := range s {
		result = append(result, k)
	}
	return result
}
