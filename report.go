package report

import (
	"errors"
	"fmt"
	//"sort"
	"context"
	"time"

	"golang.org/x/oauth2"

	"github.com/dsciamma/graphql"
)

const ISO_FORM = "2006-01-02T15:04:05Z"

// PageInfoStruct defines the structure sent by GitHub GraphQL API for Pagination
type PageInfoStruct struct {
	HasNextPage     bool
	StartCursor     string
	EndCursor       string
	HasPreviousPage bool
}

// RateLimitStruct defines the structure sent by GitHub GraphQL API for Rate limiting
type RateLimitStruct struct {
	Limit     int
	Cost      int
	Remaining int
	ResetAt   string
}

// UserStruct defines the structure sent by GitHub GraphQL API for Users
type UserStruct struct {
	Login string
}

// PRStruct defines the structure sent by GitHub GraphQL API for PullRequests
type PRStruct struct {
	Number       int
	Title        string
	Repository   string
	CreatedAt    string
	MergedAt     string
	State        string
	Participants struct {
		Nodes      []UserStruct
		PageInfo   PageInfoStruct
		TotalCount int
	}
	Timeline struct {
		TotalCount int
	}
}

// ByActivity allows to sort PRStruct by number of events
type ByActivity []PRStruct

func (a ByActivity) Len() int           { return len(a) }
func (a ByActivity) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByActivity) Less(i, j int) bool { return a[i].Timeline.TotalCount > a[j].Timeline.TotalCount }

// ByAge allows to sort PRStruct by creation date
type ByAge []PRStruct

func (a ByAge) Len() int      { return len(a) }
func (a ByAge) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByAge) Less(i, j int) bool {
	ti, _ := time.Parse(ISO_FORM, a[i].CreatedAt)
	tj, _ := time.Parse(ISO_FORM, a[j].CreatedAt)
	return tj.After(ti)
}

type repositoriesResponseStruct struct {
	Organization struct {
		Repositories struct {
			Nodes []struct {
				Name  string
				Owner UserStruct
			}
			PageInfo   PageInfoStruct
			TotalCount int
		}
	}
	RateLimit RateLimitStruct
}

type reportResponseStruct struct {
	Repository struct {
		Name     string
		MergedPR struct {
			Nodes      []PRStruct
			PageInfo   PageInfoStruct
			TotalCount int
		}
		OpenPR struct {
			Nodes      []PRStruct
			PageInfo   PageInfoStruct
			TotalCount int
		}
		Refs struct {
			Nodes []struct {
				Name   string
				Target struct {
					History struct {
						Nodes []struct {
							Oid           string
							CommittedDate string
							Author        UserStruct
							Message       string
						}
						PageInfo   PageInfoStruct
						TotalCount int
					}
				}
			}
			PageInfo   PageInfoStruct
			TotalCount int
		}
	}
	RateLimit RateLimitStruct
}

// ActivityReport object
type ActivityReport struct {
	Organization string
	Duration     int
	ReportDate   time.Time
	Result       struct {
		MergedPRs              []PRStruct
		OpenPRsWithActivity    []PRStruct
		OpenPRsWithoutActivity []PRStruct
	}

	// Log is called with various debug information.
	// To log to standard out, use:
	//  report.Log = func(s string) { log.Println(s) }
	Log func(s string)

	gitHubToken string
}

// NewActivityReport makes a new Report to extract data from GitHub.
func NewActivityReport(org string, token string, duration int) *ActivityReport {
	report := &ActivityReport{
		Organization: org,
		gitHubToken:  token,
		Duration:     duration,
	}
	return report
}

// listRepositories queries GitHub and returns the full list of repositories owned by an organization
func (gr *ActivityReport) listRepositories(
	ctx context.Context,
	client *graphql.Client,
	organization string,
	cursor string) ([]string, error) {

	var req *graphql.Request
	if cursor == "" {
		req = graphql.NewRequest(`
  query ($organization: String!, $size: Int!) {
    organization(login:$organization) {
      repositories(first:$size, affiliations:OWNER) {
        nodes {
          name
          owner {
            login
          }
        }
        pageInfo {
          hasNextPage
          endCursor
        }
        totalCount
      }
    }
    rateLimit {
      limit
      cost
      remaining
      resetAt
    }
  }
    `)
	} else {
		req = graphql.NewRequest(`
    query ($organization: String!, $size: Int!, $cursor: String!) {
      organization(login:$organization) {
        repositories(first:$size, after:$cursor) {
          nodes {
            name
          }
          pageInfo {
            hasNextPage
            endCursor
          }
          totalCount
        }
      }
      rateLimit {
        limit
        cost
        remaining
        resetAt
      }
    }
      `)
		req.Var("cursor", cursor)
	}
	req.Var("organization", organization)
	req.Var("size", 50)

	repositories := []string{}
	var respData repositoriesResponseStruct
	if err := client.Run(ctx, req, &respData); err != nil {
		return nil, err
	} else {
		for _, repo := range respData.Organization.Repositories.Nodes {
			repositories = append(repositories, repo.Name)
		}
		if respData.Organization.Repositories.PageInfo.HasNextPage {
			additionalRepos, err := gr.listRepositories(ctx, client, organization, respData.Organization.Repositories.PageInfo.EndCursor)
			if err != nil {
				return nil, err
			} else {
				repositories = append(repositories, additionalRepos...)
			}
		}
		gr.logf("Credits remaining %v\n", respData.RateLimit.Remaining)
		return repositories, nil
	}
}

// listSubsetRepositories returns a subset of repositories owned by an organization
// It's mainly used for testing purpose in order to reduce the time spent to retrieve the full list
func (gr *ActivityReport) listSubsetRepositories(
	ctx context.Context,
	client *graphql.Client,
	organization string,
	cursor string) ([]string, error) {

	var req *graphql.Request
	if cursor == "" {
		req = graphql.NewRequest(`
  query ($organization: String!, $size: Int!) {
    organization(login:$organization) {
      repositories(last:$size, affiliations:OWNER) {
        nodes {
          name
          owner {
            login
          }
        }
        pageInfo {
          hasNextPage
          endCursor
        }
        totalCount
      }
    }
    rateLimit {
      limit
      cost
      remaining
      resetAt
    }
  }
    `)
	} else {
		req = graphql.NewRequest(`
    query ($organization: String!, $size: Int!, $cursor: String!) {
      organization(login:$organization) {
        repositories(first:$size, after:$cursor) {
          nodes {
            name
          }
          pageInfo {
            hasNextPage
            endCursor
          }
          totalCount
        }
      }
      rateLimit {
        limit
        cost
        remaining
        resetAt
      }
    }
      `)
		req.Var("cursor", cursor)
	}
	req.Var("organization", organization)
	req.Var("size", 10)

	repositories := []string{}
	var respData repositoriesResponseStruct
	if err := client.Run(ctx, req, &respData); err != nil {
		return nil, err
	} else {
		for _, repo := range respData.Organization.Repositories.Nodes {
			repositories = append(repositories, repo.Name)
		}
		gr.logf("Credits remaining %v\n", respData.RateLimit.Remaining)
		return repositories, nil
	}
}

// reportRepository creates the report for 1 repository
func (gr *ActivityReport) reportRepository(
	ctx context.Context,
	client *graphql.Client,
	organization string,
	repository string,
	since time.Time) (reportResponseStruct, error) {

	// make a request
	req := graphql.NewRequest(`
query ($organization: String!, $repo: String!, $date: GitTimestamp!, $date2: DateTime!, $size: Int!) {
  repository(owner: $organization, name: $repo) {
    name
    mergedPR: pullRequests(last: $size, states: [MERGED], orderBy: {field: UPDATED_AT, direction: ASC}) {
      nodes {
        number
        title
        createdAt
        participants(last: $size) {
          nodes {
            login
          }
          totalCount
        }
        mergedAt
      }
      totalCount
    }
    openPR: pullRequests(last: $size, states: [OPEN]) {
      nodes {
        number
        title
        createdAt
        mergedAt
        state
        participants(last: $size) {
          nodes {
            login
          }
          totalCount
        }
        timeline(since: $date2) {
          totalCount
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
      totalCount
    }
    refs(refPrefix: "refs/heads/", first: $size) {
      nodes {
        ... on Ref {
          name
          target {
            ... on Commit {
              history(first: $size, since: $date) {
                nodes {
                  ... on Commit {
                    oid
                    committedDate
                    author {
                      name
                    }
                    message
                  }
                }
                pageInfo {
                  hasNextPage
                  endCursor
                }
                totalCount
              }
            }
          }
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
      totalCount
    }
  }
  rateLimit {
    limit
    cost
    remaining
    resetAt
  }
}
  `)

	// set any variables
	req.Var("organization", organization)
	req.Var("repo", repository)
	req.Var("date", since.Format(ISO_FORM))
	req.Var("date2", since.Format(ISO_FORM))
	req.Var("size", 50)

	// run it and capture the response
	var respData reportResponseStruct
	if err := client.Run(ctx, req, &respData); err != nil {
		return respData, err
	} else {
		gr.logf("Credits remaining %v\n", respData.RateLimit.Remaining)
		return respData, nil
	}
}

func (gr *ActivityReport) logf(format string, args ...interface{}) {
	gr.Log(fmt.Sprintf(format, args...))
}

// Run extracts the report from GitHub GraphQL API
func (gr *ActivityReport) Run() error {

	// create a client (safe to share across requests)
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gr.gitHubToken},
	)
	httpClient := oauth2.NewClient(ctx, tokenSource)
	client := graphql.NewClient("https://api.github.com/graphql", graphql.WithHTTPClient(httpClient), graphql.UseInlineJSON())
	//client.Log = func(s string) { fmt.Println(s) }

	now := time.Now()
	since := now.AddDate(0, 0, -gr.Duration)

	gr.ReportDate = now

	repositories, err := gr.listSubsetRepositories(ctx, client, gr.Organization, "")
	if err != nil {
		return errors.New(fmt.Sprintf("An error occured during repositories listing %v\n", err))
	} else {
		for _, repoName := range repositories {
			report, err2 := gr.reportRepository(ctx, client, gr.Organization, repoName, since)
			if err2 != nil {
				return errors.New(fmt.Sprintf("An error occured during report for %s: %v\n", repoName, err2))
			} else {
				// Build report

				// Extract Merged PR (keep the ones merged during last 7 days)
				for _, pullrequest := range report.Repository.MergedPR.Nodes {
					t, _ := time.Parse(ISO_FORM, pullrequest.MergedAt)
					if t.After(since) {
						pullrequest.Repository = repoName
						gr.Result.MergedPRs = append(gr.Result.MergedPRs, pullrequest)
					}
				}

				// Extract Open PR with and without activity
				for _, pullrequest := range report.Repository.OpenPR.Nodes {
					pullrequest.Repository = repoName
					if pullrequest.Timeline.TotalCount > 0 {
						gr.Result.OpenPRsWithActivity = append(gr.Result.OpenPRsWithActivity, pullrequest)
					} else {
						gr.Result.OpenPRsWithoutActivity = append(gr.Result.OpenPRsWithoutActivity, pullrequest)
					}
				}
			}
		}
		gr.logf("Nb merged pr:%d\n", len(gr.Result.MergedPRs))
		gr.logf("Nb open pr with activity:%d\n", len(gr.Result.OpenPRsWithActivity))
		gr.logf("Nb open pr without activity:%d\n", len(gr.Result.OpenPRsWithoutActivity))
		return nil
	}
}
