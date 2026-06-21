package pr

import (
	"strings"

	"github.com/elevran/stern/internal/github"
)

// FindBotApprovedReview returns the first APPROVED review from the bot login, or nil.
// Matches are case-insensitive on the login.
func FindBotApprovedReview(reviews []github.Review, botLogin string) *github.Review {
	for i := range reviews {
		r := &reviews[i]
		if r.State == "APPROVED" && strings.EqualFold(r.Login, botLogin) {
			return r
		}
	}
	return nil
}
