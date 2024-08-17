# migrate-archived-github-repos

Script for transferring all the archived GitHub repos in a source organisation to a target organisation.

Writes the repos which have been migrated to the file `migrated-repo-results.txt` by default (can be updated via the --results-file flag).

## Running

```shell
# Requires an access token with admin permissions on the org
export GITHUB_AUTH="<github-access-key>"

go run ./main.go --source-org "org1" --target-org "org2" [--results-file "./results.txt"]
```