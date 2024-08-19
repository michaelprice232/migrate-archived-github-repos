package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/go-github/v63/github"
)

type config struct {
	ghClient        *github.Client
	sourceGithubOrg string
	targetGithubOrg string
}

type result struct {
	originalRepoURL string
}

func main() {

	source := flag.String("source-org", "", "Source Github organisation we are moving archived repo's from")
	target := flag.String("target-org", "", "Target Github organisation we are moving archived repo's to")
	resultsFile := flag.String("results-file", "migrated-repo-results.txt", "Path that the results file will be written to")
	flag.Parse()

	if *source == "" || *target == "" {
		flag.Usage()
		log.Fatalf("Source (--source-org) and target (--target-org) Github organisations must be specified")
	}

	conf, err := newConfig(*source, *target)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	archivedRepos, err := listArchivedRepos(ctx, conf)
	if err != nil {
		log.Fatal(err)
	}

	// uncomment for testing - only target 2 repos. Also need to update the transferRepos parameter to reference subsetArchivedRepos
	//subsetArchivedRepos := make([]*github.Repository, 0)
	//for _, repo := range archivedRepos {
	//	if *repo.Name == "mike-price-test-3" || *repo.Name == "mike-price-test-4" {
	//		subsetArchivedRepos = append(subsetArchivedRepos, repo)
	//	}
	//}

	migratedRepos, err := transferRepos(ctx, conf, archivedRepos)
	if err != nil {
		log.Fatal(err)
	}

	err = writeResultsToFile(*resultsFile, migratedRepos)
	if err != nil {
		log.Fatal(err)
	}
}

// newConfig returns a config which includes the GitHub client and other required config.
func newConfig(sourceOrg, targetOrg string) (*config, error) {
	auth := os.Getenv("GITHUB_AUTH")
	if auth == "" {
		return nil, fmt.Errorf("error: GITHUB_AUTH environment variable not set")
	}

	ghClient := github.NewClient(nil).WithAuthToken(auth)

	return &config{ghClient: ghClient, sourceGithubOrg: sourceOrg, targetGithubOrg: targetOrg}, nil
}

// listArchivedRepos returns any GitHub repos in the source organisation which are currently archived.
func listArchivedRepos(ctx context.Context, c *config) ([]*github.Repository, error) {
	archivedRepos := make([]*github.Repository, 0)

	// Max page size is 100 on the API
	opt := &github.RepositoryListByOrgOptions{ListOptions: github.ListOptions{PerPage: 100}}

	for {
		repos, resp, err := c.ghClient.Repositories.ListByOrg(ctx, c.sourceGithubOrg, opt)
		if err != nil {
			return nil, fmt.Errorf("error: listing repositories in %s: %v", c.sourceGithubOrg, err)
		}

		for _, repo := range repos {
			if repo.GetArchived() {
				archivedRepos = append(archivedRepos, repo)
			}
		}

		// No more results left to page
		if resp.NextPage == 0 {
			break
		}

		// Update for the next page
		opt.Page = resp.NextPage
	}

	log.Printf("Found %d archived repositories", len(archivedRepos))

	return archivedRepos, nil
}

// transferRepos transfers all the repos from the source to target organisation.
func transferRepos(ctx context.Context, c *config, repos []*github.Repository) ([]result, error) {
	r := make([]result, 0, len(repos))

	for _, repo := range repos {
		repoName := *repo.Name

		_, resp, err := c.ghClient.Repositories.Transfer(ctx, c.sourceGithubOrg, repoName, github.TransferRequest{
			NewName:  github.String(repoName),
			NewOwner: c.targetGithubOrg,
		})
		// HTTP 202 is an expected error as the request is processed asynchronously
		if err != nil && resp != nil && resp.StatusCode != http.StatusAccepted {
			return r, fmt.Errorf("error: migrating GH repo %s from owner %s to owner %s: %v. Status code: %d", repoName, c.sourceGithubOrg, c.targetGithubOrg, err, resp.StatusCode)
		}

		log.Printf("Migrated repo %s from org %s to org %s", repoName, c.sourceGithubOrg, c.targetGithubOrg)
		r = append(r, result{originalRepoURL: *repo.HTMLURL})
	}

	return r, nil
}

// writeResultsToFile writes repos which have been migrated as results to a text file.
func writeResultsToFile(path string, results []result) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error: creating file %s failed: %v", path, err)
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Printf("error: closing file %s failed: %v", path, err)
		}
	}(f)

	for _, result := range results {
		_, err := f.WriteString(fmt.Sprintf("%s\n", result.originalRepoURL))
		if err != nil {
			return fmt.Errorf("error: writing to file %s failed: %v", path, err)
		}
	}

	return nil
}
