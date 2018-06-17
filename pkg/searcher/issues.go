package searcher

import "github.com/shurcooL/githubv4"

type issuesT struct {
	Repository struct {
		Issues struct {
			TotalCount githubv4.Int
			Nodes      []struct {
				ID        githubv4.String
				Number    githubv4.Int
				UpdatedAt githubv4.DateTime
				URL       githubv4.URI
				Title     githubv4.String
				Author    struct {
					Login githubv4.String
				}
				BodyText githubv4.String

				Comments struct {
					Nodes []struct {
						BodyText githubv4.String
						Author   struct {
							Login githubv4.String
						}
					}
				} `graphql:"comments(first:100)"`
			}
			PageInfo struct {
				EndCursor   githubv4.String
				HasNextPage githubv4.Boolean
			}
		} `graphql:"issues(first:$issuesFirst,after:$issuesCursor,orderBy:$issuesOrderBy)"`
	} `graphql:"repository(owner:$repositoryOwner,name:$repositoryName)"`
}
