package config

import (
	_ "embed"
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"
)

//go:embed template.yaml.tmpl
var configTemplate string

// templateData holds values substituted into the config template.
type templateData struct {
	Org  string
	Repo string
}

// Generate renders a commented stern.yaml template and returns it as bytes.
func Generate(org, repo string) ([]byte, error) {
	tmpl, err := template.New("stern.yaml").Parse(configTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData{Org: org, Repo: repo}); err != nil {
		return nil, fmt.Errorf("rendering template: %w", err)
	}
	return buf.Bytes(), nil
}

// OrgRepoFromGitHubRepository extracts org and repo from the GITHUB_REPOSITORY
// environment variable ("owner/repo"), falling back to the provided flags.
func OrgRepoFromGitHubRepository(flagOrg, flagRepo string) (org, repo string) {
	if flagOrg != "" && flagRepo != "" {
		return flagOrg, flagRepo
	}
	if s := os.Getenv("GITHUB_REPOSITORY"); s != "" {
		parts := strings.SplitN(s, "/", 2)
		if len(parts) == 2 {
			if flagOrg == "" {
				flagOrg = parts[0]
			}
			if flagRepo == "" {
				flagRepo = parts[1]
			}
		}
	}
	return flagOrg, flagRepo
}
