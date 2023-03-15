package merge

import (
	"context"
	"github-merge-manager/config"
	"net/http"

	"github.com/machinebox/graphql"
	"github.com/youshy/logger"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

var ctx = context.Background()

func NewGithubClientPAT(ctx context.Context, accessToken string) *graphql.Client {
	httpClient := newOauthClientAccessToken(ctx, accessToken)
	return graphql.NewClient("https://api.github.com/graphql", graphql.WithHTTPClient(httpClient))
}

func newOauthClientAccessToken(ctx context.Context, accessToken string) *http.Client {
	c := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	return oauth2.NewClient(ctx, c)
}

type Manager struct {
	client *graphql.Client
	cfg    *config.Config
	log    *zap.SugaredLogger
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

	// Define the GraphQL query for listing repositories
	queryRepos := `
	query ($orgName: String!, $cursor: String) {
	  organization(login: $orgName) {
		repositories(first: 100, after: $cursor) {
		  pageInfo {
			hasNextPage
			endCursor
		  }
		  nodes {
			name
			owner {
			  login
			}
		  }
		}
	  }
	}
	`

	// Define the GraphQL query for listing pull requests
	queryPRs := `
	query ($owner: String!, $repo: String!, $cursor: String) {
	  repository(owner: $owner, name: $repo) {
		pullRequests(first: 100, after: $cursor, states: OPEN) {
		  pageInfo {
			hasNextPage
			endCursor
		  }
		  nodes {
			title
			number
			body
			id
			headRefName
			headRepository {
			  name
			  owner {
				login
			  }
			}
		  }
		}
	  }
	}
	`

	// Define the GraphQL mutation for adding a review
	mutationAddReview := `
	mutation ($pullRequestId: ID!) {
	  addPullRequestReview(input: {
		pullRequestId: $pullRequestId,
		event: APPROVE
	  }) {
		clientMutationId
	  }
	}
	`

	// Define the GraphQL mutation for merging a pull request
	mutationMergePR := `
	mutation ($pullRequestId: ID!, $commitHeadline: String, $commitBody: String, $mergeMethod: MergeMethod) {
	  mergePullRequest(input: {
		pullRequestId: $pullRequestId,
		commitHeadline: $commitHeadline,
		commitBody: $commitBody,
		mergeMethod: $mergeMethod
	  }) {
		pullRequest {
		  title
		  number
		}
		mergeCommit {
		  oid
		}
	  }
	}
	`

	// Fetch repositories using the queryRepos
	var repos []struct {
		Name  string
		Owner struct {
			Login string
		}
	}

	cursor := ""

	for {
		req := graphql.NewRequest(queryRepos)
		req.Var("orgName", m.cfg.OrgName)
		req.Var("cursor", cursor)

		var respData struct {
			Organization struct {
				Repositories struct {
					PageInfo struct {
						HasNextPage bool
						EndCursor   string
					}
					Nodes []struct {
						Name  string
						Owner struct {
							Login string
						}
					}
				}
			}
		}

		if err := m.client.Run(ctx, req, &respData); err != nil {
			panic(err)
		}

		repos = append(repos, respData.Organization.Repositories.Nodes...)
		m.log.Infof("repos found so far: %d", len(repos))

		if !respData.Organization.Repositories.PageInfo.HasNextPage {
			break
		}
		cursor = respData.Organization.Repositories.PageInfo.EndCursor
	}

	m.log.Infof("Found %d repos", len(repos))

	// Loop through each repository and process its pull requests
	for _, repo := range repos {
		cursor = ""

		for {
			req := graphql.NewRequest(queryPRs)
			req.Var("owner", repo.Owner.Login)
			req.Var("repo", repo.Name)
			req.Var("cursor", cursor)

			var respData struct {
				Repository struct {
					PullRequests struct {
						PageInfo struct {
							HasNextPage bool
							EndCursor   string
						}
						Nodes []struct {
							Title       string
							Number      int
							Body        string
							ID          string
							HeadRefName string
							HeadRepository struct {
								Name  string
								Owner struct {
									Login string
								}
							}
						}
					}
				}
			}

			if err := m.client.Run(ctx, req, &respData); err != nil {
				panic(err)
			}

			// Process each pull request in the repository
			for _, pr := range respData.Repository.PullRequests.Nodes {
				if pr.Title == m.cfg.SubjectMatcher {
					m.log.Infof("Found PR in repository %s pr number: %d", pr.HeadRepository.Name, pr.Number)

					if slices.Contains(m.cfg.GetAction(), "approve") {
						if m.cfg.DryRun {
							m.log.Infof("dry run, skipping approving PR %d", pr.Number)
						} else {
							m.log.Debugf("approving PR %d", pr.Number)

							req := graphql.NewRequest(mutationAddReview)
							req.Var("pullRequestId", pr.ID)

							var respData struct {
								AddPullRequestReview struct {
									ClientMutationId string
								}
							}

							if err := m.client.Run(ctx, req, &respData); err != nil {
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

							req := graphql.NewRequest(mutationMergePR)
							req.Var("pullRequestId", pr.ID)
							req.Var("commitHeadline", pr.Title)
							req.Var("commitBody", cmtMsg)
							req.Var("mergeMethod", m.cfg.MergeType)

							var respData struct {
								MergePullRequest struct {
									PullRequest struct {
										Title  string
										Number int
									}
									MergeCommit struct {
										Oid string
									}
								}
							}

							if err := m.client.Run(ctx, req, &respData); err != nil {
								m.log.Errorf("Error merging PR %d: %s", pr.Number, err.Error())
							} else {
								m.log.Infof("Merged PR %d with commit %s", respData
							MergePullRequest.PullRequest.Number, respData.MergePullRequest.MergeCommit.Oid)
							}
						}
					}
				}
			}

			if !respData.Repository.PullRequests.PageInfo.HasNextPage {
				break
			}

			cursor = respData.Repository.PullRequests.PageInfo.EndCursor
		}
	}
}
