package config

import (
	"golang.org/x/exp/slices"
	"os"
	"strings"
)

var AppConfig = NewConfig()

var AllowedActions = []string{"approve", "enable-auto-merge", "force-merge"}

type Config struct {
	GithubToken    string
	OrgName        string
	DryRun         bool
	SubjectMatcher string
	Author         string
	Action         string
	MergeType      string
}

func NewConfig() *Config {
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		panic("GITHUB_TOKEN is not set")
	}
	return &Config{
		GithubToken: githubToken,
	}
}

func (c *Config) GetAction() []string {
	return strings.Split(c.Action, ",")
}

func (c *Config) Validate() {
	c.ValidateAction()
	c.ValidateMergeType()
}
func (c *Config) ValidateMergeType() {
	if !slices.Contains([]string{"merge", "squash", "rebase"}, c.MergeType) {
		panic("Invalid merge type: " + c.MergeType)
	}
}

func (c *Config) ValidateAction() {
	for _, action := range c.GetAction() {
		if !slices.Contains(AllowedActions, action) {
			panic("Invalid action: " + action)
		}
	}
}
