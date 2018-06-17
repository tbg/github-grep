package cmd

import (
	"os"

	"github.com/tschottdorf/github-grep/pkg/util"
)

const tokenEnvVar = "GHI_TOKEN"

func config() (util.Config, error) {
	org, repo, err := util.Repo(".", "origin")
	if err != nil {
		return util.Config{}, err
	}
	c := util.Config{
		DBFile:      ".git/ghg.db",
		Org:         org,
		Repo:        repo,
		AccessToken: os.Getenv(tokenEnvVar),
	}
	return c, nil
}
