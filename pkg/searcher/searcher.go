package searcher

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/gosuri/uiprogress"
	"github.com/shurcooL/githubv4"
	"github.com/tschottdorf/github-grep/pkg/util"
	"golang.org/x/oauth2"
)

const stmtCreateFTS = "CREATE VIRTUAL TABLE IF NOT EXISTS issues_fts USING fts5(title, tokens, id UNINDEXED)"

// Searcher synchronizes and retrieves GitHub issues.
type Searcher struct {
	cfg    util.Config
	issues issuesT
}

// NewSearcher initializes a new Searcher.
func NewSearcher(cfg util.Config) *Searcher {
	return &Searcher{
		cfg: cfg,
	}
}

// Num returns the number of issues in the database.
func (s *Searcher) Num() (int, error) {
	s.ensureSchema(false)

	var n int
	if err := s.cfg.DB().QueryRow(`SELECT COUNT(id) FROM issues`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// Sync updates the local database of issues from the remote. When
// 'rebuild' is true, it discards its state prior to the update and
// has to retrieve *all* issues anew.
func (s *Searcher) Sync(rebuild bool) error {
	if !flag.Parsed() {
		flag.Parse()
	}

	s.ensureSchema(rebuild)

	dateThresh, err := s.dateThreshold()
	if err != nil {
		return err
	}
	glog.Infof("looking for issues updated after %s", dateThresh)

	tty := s.cfg.IsTTY()

	var bar *uiprogress.Bar
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: s.cfg.AccessToken},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)

	variables := map[string]interface{}{
		"repositoryOwner": githubv4.String(s.cfg.Org),
		"repositoryName":  githubv4.String(s.cfg.Repo),
		"issuesFirst":     githubv4.Int(100), // max allowed by GitHub
		"issuesOrderBy": githubv4.IssueOrder{
			Field:     githubv4.IssueOrderFieldUpdatedAt,
			Direction: githubv4.OrderDirectionDesc,
		},
		"issuesCursor": (*githubv4.String)(nil),
	}

	tx, err := s.cfg.DB().Begin()
	if err != nil {
		return err
	}

	var regenerateFTS bool
	for {
		var issues issuesT
		if err := client.Query(context.Background(), &issues, variables); err != nil {
			return err
		}

		if bar == nil {
			count := int(issues.Repository.Issues.TotalCount)
			bar = uiprogress.AddBar(count).
				AppendCompleted().
				PrependElapsed().
				PrependFunc(func(b *uiprogress.Bar) string {
					return fmt.Sprintf("Issue %d/%d", b.Current(), count)
				})
			if tty {
				uiprogress.Start()
			}
		}

		for _, issue := range issues.Repository.Issues.Nodes {
			bar.Incr()

			if !issue.UpdatedAt.After(dateThresh) {
				// We're sorting issues by updated at in descending order, so
				// once we drop below the largest updated at in our database,
				// we've seen everything.
				bar.Set(bar.Total)
				issues.Repository.Issues.PageInfo.HasNextPage = false // terminate outer loop
				break
			}

			glog.Infof("issue %s new or updated at %s", issue.URL, issue.UpdatedAt)

			// The comments string includes the title and the initial post.
			// This simplifies the full text search.
			comments := []string{string(issue.Title), string(issue.BodyText)}
			for _, comment := range issue.Comments.Nodes {
				comments = append(
					comments,
					strings.Replace(string(comment.BodyText), "\n", " ", -1),
				)
			}

			if _, err := tx.Exec(
				`INSERT OR REPLACE INTO issues(id, updated_at, number, title, comments, url) VALUES($1, $2, $3, $4, $5, $6)`,
				issue.ID, issue.UpdatedAt.Time, issue.Number,
				issue.Title, strings.Join(comments, "\n\n"),
				issue.URL.String(),
			); err != nil {
				return err
			}

			regenerateFTS = true
		}

		if !issues.Repository.Issues.PageInfo.HasNextPage {
			break
		}

		variables["issuesCursor"] = githubv4.NewString(
			issues.Repository.Issues.PageInfo.EndCursor,
		)
	}

	if regenerateFTS {
		const stmt = `
DROP TABLE IF EXISTS issues_fts;
` + stmtCreateFTS + `;
INSERT INTO issues_fts SELECT title, comments, id FROM issues`
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if bar != nil && tty {
		uiprogress.Stop()
	}
	return nil
}

func (s *Searcher) ensureSchema(rebuild bool) error {
	stmts := []string{
		`
CREATE TABLE IF NOT EXISTS issues (
									id TEXT PRIMARY KEY,
									number INT NOT NULL,
									updated_at DATETIME NOT NULL,
									title TEXT,
									body TEXT,
									comments TEXT,
									url TEXT
)`,
		`CREATE INDEX IF NOT EXISTS issues_updated_at ON issues (updated_at)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS issues_by_number ON issues (number)`,
	}

	if rebuild {
		stmts = append([]string{
			`DROP TABLE IF EXISTS issues`,
			`DROP TABLE IF EXISTS issues_fts`,
		}, stmts...)
	}

	for _, stmt := range stmts {
		if _, err := s.cfg.DB().Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Searcher) dateThreshold() (time.Time, error) {
	var dateThreshold time.Time
	// NB: there's some weird behavior when using min(updated_at) instead. The timestamp
	// scans as empty, no matter what I try (including casts to datetime).
	rows, err := s.cfg.DB().Query(
		`SELECT updated_at FROM issues ORDER BY updated_at DESC LIMIT 1`,
	)
	if err != nil {
		return time.Time{}, err
	}
	defer rows.Close()
	if rows.Next() {
		if err != nil {
			return time.Time{}, err
		}
		rows.Scan(&dateThreshold)
	}
	return dateThreshold, nil
}

// Result is a match returned from Search.
type Result struct {
	IssueNumber         int
	Title, Excerpt, URL string
	Comments            string
}

// Search looks for the given query, returning up to the specified number of
// Results. The query is a sqlite-fts5 MATCH invocation. See:
// https://www.sqlite.org/fts5.html
func (s *Searcher) Search(query string, max int) ([]Result, error) {
	s.ensureSchema(false)
	const match = `
SELECT issues.number, a.title, a.excerpt, a.tokens, issues.url FROM (
  SELECT id, title, snippet(issues_fts, -1, '', '', '…', 16) excerpt, tokens FROM issues_fts WHERE tokens MATCH $1 ORDER BY rank LIMIT $2) a JOIN issues USING(id);
`
	rows, err := s.cfg.DB().Query(match, query, max)
	if err != nil {
		return nil, err
	}

	var results []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.IssueNumber, &r.Title, &r.Excerpt, &r.Comments, &r.URL); err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	return results, nil
}
