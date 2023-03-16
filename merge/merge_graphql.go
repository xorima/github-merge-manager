package merge

import (
	"context"
	"github-merge-manager/config"
	"net/http"

	"github.com/shurcooL/githubv4"
	"github.com/youshy/logger"
	"golang.org/x/exp/slices"
	"golang.org/x/oauth2"
)

var ctx = context.Background()

func NewGithubClientPAT(ctx context.Context, accessToken string) *githubv4.Client {
	httpClient := newOauthClientAccessToken(ctx, accessToken)
	return githubv4.NewClient(httpClient)
}

func newOauthClientAccessToken(ctx context.Context, accessToken string) *http.Client {
	c := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	return oauth2.NewClient(ctx, c)
}

func NewManager(cfg *config.Config) *Manager {
	client := NewGithubClientPAT(ctx, cfg.GithubToken)
	log := logger.NewLogger(logger.INFO, false)
	return &Manager{
		client: client,
		cfg:    cfg,
		log:    log,
	}
}

func (m *Manager) Handle() {
	m.log.Infof("running with options: %+v", m.cfg.GetAction())
	m.log.Infof("searching for PRs in org %s with title `%s`", m.cfg.OrgName, m.cfg.SubjectMatcher)
	m.log.Infof("merge method defined as %s", m.cfg.MergeType)
	m.log.Infof("dryrun is %t", m.cfg.DryRun)

	var queryRepos struct {
		Organization struct {
			Repositories struct {
				PageInfo struct {
					HasNextPage bool
					EndCursor   githubv4.String
				}
				Nodes []Repository
			} `graphql:"repositories(first: 100, after: $cursor)"`
		} `graphql:"organization(login: $orgName)"`
	}

	var queryPRs struct {
		Repository struct {
			PullRequests struct {
				PageInfo struct {
					HasNextPage bool
					EndCursor   githubv4.String
				}
				Nodes []PullRequest
			} `graphql:"pullRequests(first: 100, after: $cursor, states: OPEN)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	var mutationAddReview struct {
		AddPullRequestReview struct {
			ClientMutationID githubv4.String
		} `graphql:"addPullRequestReview(input: $input)"`
	}

	var mutationMergePR struct {
		MergePullRequest struct {
			PullRequest struct {
				Title  string
				Number int
			}
			MergeCommit struct {
				Oid string
			}
		} `graphql:"mergePullRequest(input: $input)"`
	}

	// Fetch repositories using GraphQL query
	var repos []Repository
	variables := map[string]interface{}{
		"orgName": githubv4.String(m.cfg.OrgName),
		"cursor":  (*githubv4.String)(nil),
	}

	for {
		if err := m.client.Query(ctx, &queryRepos, variables); err != nil {
			panic(err)
		}

		repos = append(repos, queryRepos.Organization.Repositories.Nodes...)

		if !queryRepos.Organization.Repositories.PageInfo.HasNextPage {
			break
		}

		variables["cursor"] = queryRepos.Organization.Repositories.PageInfo.EndCursor
	}

	m.log.Infof("Found %d repos", len(repos))

	// Loop through each repository and process its pull requests
	for _, repo := range repos {
		variables := map[string]interface{}{
			"owner":  githubv4.String(repo.Owner.Login),
			"repo":   githubv4.String(repo.Name),
			"cursor": (*githubv4.String)(nil),
		}

		for {
			if err := m.client.Query(ctx, &queryPRs, variables); err != nil {
				panic(err)
			}

			// Process each pull request in the repository
			for _, pr := range queryPRs.Repository.PullRequests.Nodes {
				if pr.Title == m.cfg.SubjectMatcher {
					m.log.Infof("Found PR in repository %s pr number: %d", pr.HeadRepository.Name, pr.Number)

					if slices.Contains(m.cfg.GetAction(), "approve") {
						if m.cfg.DryRun {
							m.log.Infof("dry run, skipping approving PR %d", pr.Number)
						} else {
							m.log.Debugf("approving PR %d", pr.Number)

							input := AddPullRequestReviewInput{
								PullRequestID: pr.ID,
								Event:         "APPROVE",
							}

							err := m.client.Mutate(ctx, &mutationAddReview, input, nil)
							if err != nil {
								m.log.Errorf("error approving PR %d: %s", pr.Number, err.Error())
								m.log.Warnf("skipping any other actions for this pull request %d", pr.Number)
								continue
							}
						}
					}

					if slices.Contains(m.cfg.GetAction(), "force-merge") {
						if m.cfg.DryRun {
							m.log.Infof("dry run, skipping merge PR %d", pr.Number)
						} else {
							m.log.Debugf("merging PR %d", pr.Number)

							prefix := m.cfg.MergeMsgPrefix
							if prefix[len(prefix)-1:] != " " {
								prefix = prefix + " "
							}
							cmtMsg := prefix + pr.Body

							input := MergePullRequestInput{
								PullRequestID:  pr.ID,
								CommitHeadline: pr.Title,
								CommitBody:     cmtMsg,
								MergeMethod:    m.cfg.MergeType,
							}

							err := m.client.Mutate(ctx, &mutationMergePR, input, nil)
							if err != nil {
								m.log.Error(err)
							}
							m.log.Infof("Merge result: %v", mutationMergePR.MergePullRequest.MergeCommit.Oid)
						}
					}
				}
			}

			if !queryPRs.Repository.PullRequests.PageInfo.HasNextPage {
				break
			}

			variables["cursor"] = queryPRs.Repository.PullRequests.PageInfo.EndCursor
		}
	}
}
