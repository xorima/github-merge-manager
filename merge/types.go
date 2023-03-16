package merge

import (
	"github-merge-manager/config"

	"github.com/shurcooL/githubv4"
	"go.uber.org/zap"
)

// Define the GraphQL mutation for adding a review
type AddPullRequestReviewInput struct {
	PullRequestID githubv4.ID `json:"pullRequestId"`
	Event         string      `json:"event"`
}

type Manager struct {
	client *githubv4.Client
	cfg    *config.Config
	log    *zap.SugaredLogger
}

// Define the GraphQL mutation for merging a pull request
type MergePullRequestInput struct {
	PullRequestID  githubv4.ID `json:"pullRequestId"`
	CommitHeadline string      `json:"commitHeadline"`
	CommitBody     string      `json:"commitBody"`
	MergeMethod    string      `json:"mergeMethod"`
}

// Define the GraphQL query for listing pull requests
type PullRequest struct {
	Title          string
	Number         int
	Body           string
	ID             githubv4.ID
	HeadRefName    string
	HeadRepository struct {
		Name  string
		Owner struct {
			Login string
		}
	}
}

// Define the GraphQL query for listing repositories
type Repository struct {
	Name  string
	Owner struct {
		Login string
	}
}
