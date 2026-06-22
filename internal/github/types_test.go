package github

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	gh "github.com/google/go-github/v72/github"
	"github.com/stretchr/testify/assert"
)

func TestIsNotFoundError(t *testing.T) {
	notFoundResp := &http.Response{StatusCode: http.StatusNotFound}
	otherResp := &http.Response{StatusCode: http.StatusInternalServerError}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"plain error", errors.New("something"), false},
		{"404 with response", &gh.ErrorResponse{Response: notFoundResp, Message: "Not Found"}, true},
		{"500 with response", &gh.ErrorResponse{Response: otherResp, Message: "Server Error"}, false},
		{"ErrorResponse with nil response", &gh.ErrorResponse{Message: "no response"}, false},
		{"wrapped 404", fmt.Errorf("layered: %w", &gh.ErrorResponse{Response: notFoundResp}), true},
		{"wrapped 500", fmt.Errorf("layered: %w", &gh.ErrorResponse{Response: otherResp}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsNotFoundError(tt.err))
		})
	}
}

func TestPullRequestFromGH_Nil(t *testing.T) {
	// Nil receiver must not panic and must produce a zero-valued PullRequest.
	got := PullRequestFromGH(nil)
	assert.Equal(t, PullRequest{}, got)
}

func TestPullRequestFromGH_FullyPopulated(t *testing.T) {
	pr := &gh.PullRequest{
		Number:         gh.Ptr(42),
		Title:          gh.Ptr("add thing"),
		NodeID:         gh.Ptr("PR_node"),
		Draft:          gh.Ptr(false),
		Additions:      gh.Ptr(10),
		Deletions:      gh.Ptr(5),
		Merged:         gh.Ptr(true),
		MergeCommitSHA: gh.Ptr("deadbeef"),
		Labels: []*gh.Label{
			{Name: gh.Ptr("lgtm")},
			{Name: gh.Ptr("approved")},
		},
		Head: &gh.PullRequestBranch{SHA: gh.Ptr("head_sha")},
		Base: &gh.PullRequestBranch{
			SHA: gh.Ptr("base_sha"),
			Ref: gh.Ptr("main"),
		},
		User: &gh.User{Login: gh.Ptr("alice")},
	}

	got := PullRequestFromGH(pr)
	assert.Equal(t, 42, got.Number)
	assert.Equal(t, "add thing", got.Title)
	assert.Equal(t, "PR_node", got.NodeID)
	assert.False(t, got.IsDraft)
	assert.Equal(t, 10, got.Additions)
	assert.Equal(t, 5, got.Deletions)
	assert.True(t, got.Merged)
	assert.Equal(t, "deadbeef", got.MergeCommitSHA)
	assert.Equal(t, []string{"lgtm", "approved"}, got.Labels)
	assert.Equal(t, "head_sha", got.HeadSHA)
	assert.Equal(t, "base_sha", got.BaseSHA)
	assert.Equal(t, "main", got.BaseRef)
	assert.Equal(t, "alice", got.Author)
}

func TestPullRequestFromGH_NilPointersInsideFields(t *testing.T) {
	// PR with all sub-pointers nil — exercises the guard checks for Head/Base/User.
	pr := &gh.PullRequest{
		Number: gh.Ptr(7),
		Title:  gh.Ptr("bare"),
	}
	got := PullRequestFromGH(pr)
	assert.Equal(t, 7, got.Number)
	assert.Equal(t, "bare", got.Title)
	assert.Empty(t, got.HeadSHA)
	assert.Empty(t, got.BaseSHA)
	assert.Empty(t, got.BaseRef)
	assert.Empty(t, got.Author)
	assert.Empty(t, got.Labels)
}

func TestLabelToGH(t *testing.T) {
	l := Label{Name: "lgtm", Color: "0e8a16", Description: "Looks good to me"}
	got := labelToGH(l)
	if assert.NotNil(t, got) {
		assert.Equal(t, "lgtm", got.GetName())
		assert.Equal(t, "0e8a16", got.GetColor())
		assert.Equal(t, "Looks good to me", got.GetDescription())
	}
}
