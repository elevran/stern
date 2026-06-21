package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	gh "github.com/google/go-github/v72/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func newTestClient(t *testing.T, srv *httptest.Server) *realClient {
	t.Helper()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})
	tc := oauth2.NewClient(context.Background(), ts)
	ghc, err := gh.NewClient(tc).WithAuthToken("test-token").WithEnterpriseURLs(srv.URL+"/", srv.URL+"/")
	require.NoError(t, err)
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
	require.NoError(t, c.EnableAutoMerge(context.Background(), "PR_abc", "squash"))
	assert.Contains(t, captured.Query, "enablePullRequestAutoMerge")
	assert.Equal(t, "PR_abc", captured.Variables["id"])
	assert.Equal(t, "SQUASH", captured.Variables["method"])
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
	require.NoError(t, c.DisableAutoMerge(context.Background(), "PR_xyz"))
	assert.Contains(t, captured.Query, "disablePullRequestAutoMerge")
	assert.Equal(t, "PR_xyz", captured.Variables["id"])
}

func TestEnableAutoMerge_GraphQLError_Propagated(t *testing.T) {
	srv, _ := captureGraphQL(t, map[string]any{
		"data": nil,
		"errors": []map[string]any{
			{"message": "Pull request Pull request is in clean status", "type": "UNPROCESSABLE"},
		},
	})

	c := newTestClient(t, srv)
	err := c.EnableAutoMerge(context.Background(), "PR_abc", "squash")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clean status")
	var gqlErr *GraphQLError
	require.ErrorAs(t, err, &gqlErr, "expected error to be a *GraphQLError")
	assert.Equal(t, "UNPROCESSABLE", gqlErr.Type)
	assert.Equal(t, "Pull request Pull request is in clean status", gqlErr.Message)
}

// paginatedLabelsServer returns labels in two pages: page 1 returns
// `page1Labels` with a Link header pointing to page 2; page 2 returns
// `page2Labels` with no Link header (terminal page).
func paginatedLabelsServer(t *testing.T, page1Labels, page2Labels []map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/repos/o/r/labels" {
			http.NotFound(w, r)
			return
		}
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "", "1":
			link := `<http://` + r.Host + `/api/v3/repos/o/r/labels?page=2>; rel="next"`
			w.Header().Set("Link", link)
			_ = json.NewEncoder(w).Encode(page1Labels)
		case "2":
			_ = json.NewEncoder(w).Encode(page2Labels)
		default:
			t.Errorf("unexpected page request: %q", page)
			http.Error(w, "unexpected page", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestListRepoLabels_Pagination(t *testing.T) {
	page1 := []map[string]any{
		{"name": "bug", "color": "f00", "description": "something is broken"},
		{"name": "feature", "color": "0f0", "description": "new feature"},
	}
	page2 := []map[string]any{
		{"name": "wontfix", "color": "000", "description": "out of scope"},
	}
	srv := paginatedLabelsServer(t, page1, page2)

	c := newTestClient(t, srv)
	labels, err := c.ListRepoLabels(context.Background(), "o", "r")
	require.NoError(t, err)
	require.Len(t, labels, 3, "expected 3 labels across two pages")
	names := make(map[string]bool)
	for _, l := range labels {
		names[l.Name] = true
	}
	assert.Equal(t, map[string]bool{"bug": true, "feature": true, "wontfix": true}, names, "expected all labels to be present")
}

func TestListPullRequestFiles_Pagination(t *testing.T) {
	page1 := []map[string]any{
		{"filename": "main.go"},
		{"filename": "cmd/run.go"},
	}
	page2 := []map[string]any{
		{"filename": "internal/x/y.go"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/api/v3/repos/o/r/pulls/1/files"
		if r.URL.Path != expected {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		page := r.URL.Query().Get("page")
		switch page {
		case "", "1":
			w.Header().Set("Link", `<http://`+r.Host+expected+`?page=2>; rel="next"`)
			_ = json.NewEncoder(w).Encode(page1)
		case "2":
			_ = json.NewEncoder(w).Encode(page2)
		default:
			http.Error(w, "unexpected page", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv)
	files, err := c.ListPullRequestFiles(context.Background(), "o", "r", 1)
	require.NoError(t, err)
	want := map[string]bool{"main.go": true, "cmd/run.go": true, "internal/x/y.go": true}
	got := make(map[string]bool)
	for _, f := range files {
		got[f] = true
	}
	assert.Equal(t, want, got, "expected all files to be present across two pages")
}
