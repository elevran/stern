package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gh "github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"
)

func newTestClient(t *testing.T, srv *httptest.Server) *realClient {
	t.Helper()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})
	tc := oauth2.NewClient(context.Background(), ts)
	ghc, err := gh.NewClient(tc).WithAuthToken("test-token").WithEnterpriseURLs(srv.URL+"/", srv.URL+"/")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return &realClient{ghc: ghc}
}

// captureGraphQL returns a server that captures the last GraphQL request body.
func captureGraphQL(t *testing.T, response any) (*httptest.Server, *graphqlRequest) {
	t.Helper()
	captured := &graphqlRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/graphql" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, captured)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

func TestEnableAutoMerge_SendsCorrectMutation(t *testing.T) {
	srv, captured := captureGraphQL(t, map[string]any{
		"data": map[string]any{
			"enablePullRequestAutoMerge": map[string]any{
				"pullRequest": map[string]any{"id": "PR_abc"},
			},
		},
	})

	c := newTestClient(t, srv)
	if err := c.EnableAutoMerge(context.Background(), "PR_abc", "squash"); err != nil {
		t.Fatalf("EnableAutoMerge() error = %v", err)
	}
	if !strings.Contains(captured.Query, "enablePullRequestAutoMerge") {
		t.Errorf("query does not contain enablePullRequestAutoMerge: %q", captured.Query)
	}
	if captured.Variables["id"] != "PR_abc" {
		t.Errorf("variable id = %v, want PR_abc", captured.Variables["id"])
	}
	if captured.Variables["method"] != "SQUASH" {
		t.Errorf("variable method = %v, want SQUASH", captured.Variables["method"])
	}
}

func TestDisableAutoMerge_SendsCorrectMutation(t *testing.T) {
	srv, captured := captureGraphQL(t, map[string]any{
		"data": map[string]any{
			"disablePullRequestAutoMerge": map[string]any{
				"pullRequest": map[string]any{"id": "PR_xyz"},
			},
		},
	})

	c := newTestClient(t, srv)
	if err := c.DisableAutoMerge(context.Background(), "PR_xyz"); err != nil {
		t.Fatalf("DisableAutoMerge() error = %v", err)
	}
	if !strings.Contains(captured.Query, "disablePullRequestAutoMerge") {
		t.Errorf("query does not contain disablePullRequestAutoMerge: %q", captured.Query)
	}
	if captured.Variables["id"] != "PR_xyz" {
		t.Errorf("variable id = %v, want PR_xyz", captured.Variables["id"])
	}
}

func TestEnableAutoMerge_GraphQLError_Propagated(t *testing.T) {
	srv, _ := captureGraphQL(t, map[string]any{
		"data": nil,
		"errors": []map[string]any{
			{"message": "Pull request Pull request is in clean status"},
		},
	})

	c := newTestClient(t, srv)
	err := c.EnableAutoMerge(context.Background(), "PR_abc", "squash")
	if err == nil {
		t.Error("expected error for GraphQL error response, got nil")
	}
	if !strings.Contains(err.Error(), "clean status") {
		t.Errorf("error = %v, want it to contain 'clean status'", err)
	}
}
