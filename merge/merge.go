package merge

import (
	"context"
	"github-merge-manager/config"
	"github.com/google/go-github/v50/github"
	"github.com/youshy/logger"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"golang.org/x/oauth2"
	"net/http"
)

var ctx = context.Background()

func NewGithubClientPAT(ctx context.Context, accessToken string) *github.Client {
	httpClient := newOauthClientAccessToken(ctx, accessToken)
	return github.NewClient(httpClient)
}

func newOauthClientAccessToken(ctx context.Context, accessToken string) *http.Client {
	c := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)
	return oauth2.NewClient(ctx, c)
}

type Manager struct {
	client *github.Client
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
	var repos []*github.Repository
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	rep, r, err := m.client.Repositories.ListByOrg(ctx, m.cfg.OrgName, opts)
	if err != nil {
		panic(err)
	}
	repos = append(repos, rep...)
	m.log.Infof("repos found so far: %d", len(repos))
	for r.NextPage != 0 {
		opts.Page = r.NextPage
		rep, r, err = m.client.Repositories.ListByOrg(ctx, m.cfg.OrgName, opts)
		if err != nil {
			panic(err)
		}
		repos = append(repos, rep...)
		m.log.Infof("repos found so far: %d", len(repos))

	}
	m.log.Infof("Found %d repos", len(repos))
	var prs []*github.PullRequest
	for _, repo := range repos {
		opts := &github.PullRequestListOptions{
			State: "open",
		}
		pr, r, err := m.client.PullRequests.List(ctx, repo.GetOwner().GetLogin(), repo.GetName(), opts)
		if err != nil {
			panic(err)
		}
		prs = append(prs, pr...)
		m.log.Infof("prs found so far: %d", len(prs))

		for r.NextPage != 0 {
			opts.Page = r.NextPage
			pr, r, err = m.client.PullRequests.List(ctx, repo.GetOwner().GetLogin(), repo.GetName(), opts)
			if err != nil {
				panic(err)
			}
			prs = append(prs, pr...)
			m.log.Infof("prs found so far: %d", len(prs))

		}
	}

	m.log.Infof("Found %d PRs", len(prs))
	// for each PR, check if it has the given subject
	for _, pr := range prs {

		if pr.GetTitle() == m.cfg.SubjectMatcher {
			m.log.Infof("Found PR in org %s pr number: %d", pr.GetHead().GetRepo().GetName(), pr.GetNumber())
			if slices.Contains(m.cfg.GetAction(), "approve") {
				if m.cfg.DryRun {
					m.log.Infof("dry run, skipping approving PR %d", pr.GetNumber())
				} else {
					m.log.Debugf("approving PR %d", pr.GetNumber())
					_, _, err := m.client.PullRequests.CreateReview(ctx, m.cfg.OrgName, pr.GetHead().GetRepo().GetName(), pr.GetNumber(), &github.PullRequestReviewRequest{
						Event: github.String("APPROVE"),
					})
					if err != nil {
						m.log.Errorf("error approving PR %d: %s", pr.GetNumber(), err.Error())
						m.log.Warnf("skipping any other actions for this pull request %d", pr.GetNumber())
						continue
					}
				}
			}

			// Does not work, requires changing to the graphql api
			//if slices.Contains(m.cfg.GetAction(), "enable-auto-merge") {ƒ
			//	m.client.PullRequests.Edit(ctx, m.cfg.OrgName, pr.GetHead().GetRepo().GetName(), pr.GetNumber(), &github.PullRequest{
			//		AutoMerge: &github.PullRequestAutoMerge{
			//			MergeMethod:   github.String("squash"),
			//			CommitTitle:   github.String(pr.GetTitle()),
			//			CommitMessage: github.String(pr.GetBody()),
			//		},
			//	})
			//}
			if slices.Contains(m.cfg.GetAction(), "force-merge") {
				if m.cfg.DryRun {
					m.log.Infof("dry run, skipping merge PR %d", pr.GetNumber())
				} else {
					m.log.Debugf("merging PR %d", pr.GetNumber())
					res, _, err := m.client.PullRequests.Merge(ctx, m.cfg.OrgName, pr.GetHead().GetRepo().GetName(), pr.GetNumber(), pr.GetBody(), &github.PullRequestOptions{
						CommitTitle: pr.GetTitle(),
						MergeMethod: m.cfg.MergeType,
					})
					if err != nil {
						m.log.Error(err)
					}
					m.log.Infof("Merge result: %v", res.GetMessage())
				}
			}
		}
	}
}
