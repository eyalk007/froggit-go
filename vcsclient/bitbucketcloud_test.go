package vcsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jfrog/froggit-go/vcsutils"
	"github.com/ktrysmt/go-bitbucket"
	"github.com/stretchr/testify/assert"
)

func TestBitbucketCloud_Connection(t *testing.T) {
	ctx := context.Background()
	mockResponse := map[string][]bitbucket.User{"values": {}}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockResponse, "/user", createBitbucketCloudHandler)
	defer cleanUp()

	err := client.TestConnection(ctx)
	assert.NoError(t, err)
}

func TestBitbucketCloud_ConnectionWhenContextCancelled(t *testing.T) {
	t.Skip("Bitbucket cloud does not use the context")
	ctx := context.Background()
	ctxWithCancel, cancel := context.WithCancel(ctx)
	cancel()

	client, cleanUp := createWaitingServerAndClient(t, vcsutils.BitbucketCloud, 0)
	defer cleanUp()
	err := client.TestConnection(ctxWithCancel)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestBitbucketCloud_ConnectionWhenContextTimesOut(t *testing.T) {
	t.Skip("Bitbucket cloud does not use the context")
	ctx := context.Background()
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	client, cleanUp := createWaitingServerAndClient(t, vcsutils.BitbucketCloud, 50*time.Millisecond)
	defer cleanUp()
	err := client.TestConnection(ctxWithTimeout)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestBitbucketCloud_ListRepositories(t *testing.T) {
	ctx := context.Background()
	mockResponse := map[string][]bitbucket.Repository{
		"values": {{Slug: repo1}, {Slug: repo2}},
	}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockResponse, "/repositories/"+username, createBitbucketCloudHandler)
	defer cleanUp()

	actualRepositories, err := client.ListRepositories(ctx)
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{username: {repo1, repo2}}, actualRepositories)
}

func TestBitbucketCloud_ListBranches(t *testing.T) {
	ctx := context.Background()
	mockResponse := map[string][]bitbucket.BranchModel{
		"values": {{Name: branch1}, {Name: branch2}},
	}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockResponse, "/repositories/jfrog/repo-1/refs/branches?", createBitbucketCloudHandler)
	defer cleanUp()

	actualRepositories, err := client.ListBranches(ctx, owner, repo1)
	assert.NoError(t, err)
	assert.ElementsMatch(t, actualRepositories, []string{branch1, branch2})
}

func TestBitbucketCloud_CreateWebhook(t *testing.T) {
	ctx := context.Background()
	id, err := uuid.NewUUID()
	assert.NoError(t, err)
	mockResponse := bitbucket.WebhooksOptions{Uuid: "{" + id.String() + "}"}
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, mockResponse, "/repositories/jfrog/repo-1/hooks", createBitbucketCloudHandler)
	defer cleanUp()

	actualID, token, err := client.CreateWebhook(ctx, owner, repo1, branch1, "https://httpbin.org/anything",
		vcsutils.Push)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, id.String(), actualID)
}

func TestBitbucketCloud_UpdateWebhook(t *testing.T) {
	ctx := context.Background()
	id, err := uuid.NewUUID()
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, make(map[string]interface{}), fmt.Sprintf("/repositories/jfrog/repo-1/hooks/%s", id.String()), createBitbucketCloudHandler)
	defer cleanUp()

	err = client.UpdateWebhook(ctx, owner, repo1, branch1, "https://httpbin.org/anything", token, id.String(),
		vcsutils.PrOpened, vcsutils.PrEdited, vcsutils.PrRejected, vcsutils.PrMerged, vcsutils.TagPushed, vcsutils.TagRemoved)
	assert.NoError(t, err)
}

func TestBitbucketCloud_DeleteWebhook(t *testing.T) {
	ctx := context.Background()
	id, err := uuid.NewUUID()
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, fmt.Sprintf("/repositories/jfrog/repo-1/hooks/%s", id.String()), createBitbucketCloudHandler)

	defer cleanUp()

	err = client.DeleteWebhook(ctx, owner, repo1, id.String())
	assert.NoError(t, err)
}

func TestBitbucketCloud_SetCommitStatus(t *testing.T) {
	ctx := context.Background()
	ref := "9caf1c431fb783b669f0f909bd018b40f2ea3808"
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, fmt.Sprintf("/repositories/jfrog/repo-1/commit/%s/statuses/build", ref), createBitbucketCloudHandler)
	defer cleanUp()

	err := client.SetCommitStatus(ctx, Pass, owner, repo1, ref, "Commit status title", "Commit status description",
		"https://httpbin.org/anything")
	assert.NoError(t, err)
}

func TestBitbucketCloud_DownloadRepository(t *testing.T) {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "")
	assert.NoError(t, err)
	defer func() { assert.NoError(t, vcsutils.RemoveTempDir(dir)) }()

	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	err = client.DownloadRepository(ctx, owner, "jfrog-setup-cli", "master", dir)
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, "README.md"))
	assert.DirExists(t, filepath.Join(dir, ".git"))
}

func TestBitbucketCloud_CreatePullRequest(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, "/repositories/jfrog/repo-1/pullrequests/", createBitbucketCloudHandler)
	defer cleanUp()

	err := client.CreatePullRequest(ctx, owner, repo1, branch1, branch2, "PR title", "PR body")
	assert.NoError(t, err)
}

func TestBitbucketCloudClient_UpdatePullRequest(t *testing.T) {
	ctx := context.Background()
	prId := 3
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, fmt.Sprintf("/repositories/jfrog/repo-1/pullrequests/%v", prId), createBitbucketCloudHandler)
	defer cleanUp()

	err := client.UpdatePullRequest(ctx, owner, repo1, "PR title", "PR body", "master", prId, vcsutils.Open)
	assert.NoError(t, err)
}

func TestBitbucketCloud_ListOpenPullRequests(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "pull_requests_list_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/?state=OPEN", owner, repo1), createBitbucketCloudHandler)
	defer cleanUp()

	result, err := client.ListOpenPullRequests(ctx, owner, repo1)

	assert.NoError(t, err)
	assert.Len(t, result, 3)
	assert.EqualValues(t, PullRequestInfo{
		ID:     3,
		Title:  "A change",
		Author: "user",
		Source: BranchInfo{Name: "test-2", Repository: "user17/test"},
		Target: BranchInfo{Name: "master", Repository: "user17/test"},
	}, result[0])

	// With Body
	result, err = client.ListOpenPullRequestsWithBody(ctx, owner, repo1)

	assert.NoError(t, err)
	assert.Len(t, result, 3)
	assert.EqualValues(t, PullRequestInfo{
		ID:     3,
		Title:  "A change",
		Author: "user",
		Body:   "hello world",
		Source: BranchInfo{Name: "test-2", Repository: "user17/test"},
		Target: BranchInfo{Name: "master", Repository: "user17/test"},
	}, result[0])
}

func TestBitbucketCloudClient_GetPullRequest(t *testing.T) {
	pullRequestId := 1
	repoName := "froggit"
	ctx := context.Background()

	// Successful Response
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "get_pull_request_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%d", owner, repoName, pullRequestId), createBitbucketCloudHandler)
	defer cleanUp()
	result, err := client.GetPullRequestByID(ctx, owner, repoName, pullRequestId)
	assert.NoError(t, err)
	assert.EqualValues(t, PullRequestInfo{
		ID:     int64(pullRequestId),
		Title:  "s",
		Author: "fname lname",
		Source: BranchInfo{Name: "pr", Repository: "froggit", Owner: "forkedWorkspace"},
		Target: BranchInfo{Name: "main", Repository: "froggit", Owner: "workspace"},
	}, result)

	// Bad Response
	badClient, badClientCleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, "{",
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%d", owner, repoName, pullRequestId), createBitbucketCloudHandler)
	defer badClientCleanUp()
	_, err = badClient.GetPullRequestByID(ctx, owner, repoName, pullRequestId)
	assert.Error(t, err)

	// Bad Fields
	badRepoName := ""
	badParseClient, badParseClientCleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%d", owner, badRepoName, pullRequestId), createBitbucketCloudHandler)
	defer badParseClientCleanUp()
	_, err = badParseClient.GetPullRequestByID(ctx, owner, badRepoName, pullRequestId)
	assert.Error(t, err)

}

func TestBitbucketCloud_AddPullRequestComment(t *testing.T) {
	ctx := context.Background()
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, "/repositories/jfrog/repo-1/pullrequests/1/comments", createBitbucketCloudHandler)
	defer cleanUp()

	err := client.AddPullRequestComment(ctx, owner, repo1, "Comment content", 1)
	assert.NoError(t, err)
}

func TestBitbucketCloud_ListPullRequestComments(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "pull_request_comments_list_response.json"))
	assert.NoError(t, err)
	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/1/comments/", owner, repo1), createBitbucketCloudHandler)
	defer cleanUp()

	result, err := client.ListPullRequestComments(ctx, owner, repo1, 1)

	assert.NoError(t, err)
	expectedCreated, err := time.Parse(time.RFC3339, "2022-05-16T11:04:07.075827+00:00")
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, CommentInfo{
		ID:      301545835,
		Content: "I’m a comment ",
		Created: expectedCreated,
	}, result[0])
}

func TestBitbucketCloud_GetLatestCommit(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "commit_list_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/commits/%s?pagelen=1", owner, repo1, "master"), createBitbucketCloudHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")

	assert.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          "ec05bacb91d757b4b6b2a11a0676471020e89fb5",
		AuthorName:    "user",
		CommitterName: "",
		Url:           "https://api.bitbucket.org/2.0/repositories/user2/setup-jfrog-cli/commit/ec05bacb91d757b4b6b2a11a0676471020e89fb5",
		Timestamp:     1591040823,
		Message:       "Fix README.md: yaml\n",
		ParentHashes:  []string{"774aa0fb252bccbc2a7e01060ef4d4be0b0eeaa9", "def26c6128ebe11fac555fe58b59227e9655dc4d"},
	}, result)
}

func TestBitbucketCloud_GetCommits(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "commit_list_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/commits/%s?pagelen=1", owner, repo1, "master"), createBitbucketCloudHandler)
	defer cleanUp()

	result, err := client.GetCommits(ctx, owner, repo1, "master")
	assert.Error(t, err)
	assert.Equal(t, errBitbucketGetCommitsNotSupported.Error(), err.Error())
	assert.Nil(t, result)
}

func TestBitbucketCloud_GetLatestCommitNotFound(t *testing.T) {
	ctx := context.Background()
	response := []byte(`<!DOCTYPE html><html lang="en"></html>`)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/commits/%s?pagelen=1", owner, repo1, "master"), http.StatusNotFound,
		createBitbucketCloudHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "master")
	assert.EqualError(t, err, "404 Not Found")
	assert.Empty(t, result)
}

func TestBitbucketCloud_GetLatestCommitUnknownBranch(t *testing.T) {
	ctx := context.Background()
	response := []byte(`{
		"data": {
			"shas": [
				"unknown"
			]
		},
		"error": {
			"data": {
				"shas": [
					"unknown"
				]
			},
			"message": "Commit not found"
		},
		"type": "error"
	}`)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/commits/%s?pagelen=1", owner, repo1, "unknown"), http.StatusNotFound,
		createBitbucketCloudHandler)
	defer cleanUp()

	result, err := client.GetLatestCommit(ctx, owner, repo1, "unknown")
	assert.EqualError(t, err, "404 Not Found")
	assert.Empty(t, result)
}

func TestBitbucketCloud_AddSshKeyToRepository(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "add_ssh_key_response.json"))
	assert.NoError(t, err)

	expectedBody := []byte(`{"key":"ssh-rsa AAAA...","label":"My deploy key"}` + "\n")

	client, closeServer := createBodyHandlingServerAndClient(t, vcsutils.BitbucketCloud, true,
		response, fmt.Sprintf("/repositories/%s/%s/deploy-keys", owner, repo1), http.StatusOK,
		expectedBody, http.MethodPost,
		createBitbucketCloudWithBodyHandler)
	defer closeServer()

	err = client.AddSshKeyToRepository(ctx, owner, repo1, "My deploy key", "ssh-rsa AAAA...", Read)

	assert.NoError(t, err)
}

func TestBitbucketCloud_AddSshKeyToRepositoryNotFound(t *testing.T) {
	ctx := context.Background()
	response := []byte(`The requested repository either does not exist or you do not have access. If you believe this repository exists and you have access, make sure you're authenticated.`)

	expectedBody := []byte(`{"key":"ssh-rsa AAAA...","label":"My deploy key"}` + "\n")

	client, closeServer := createBodyHandlingServerAndClient(t, vcsutils.BitbucketCloud, true,
		response, fmt.Sprintf("/repositories/%s/%s/deploy-keys", owner, repo1), http.StatusNotFound,
		expectedBody, http.MethodPost,
		createBitbucketCloudWithBodyHandler)
	defer closeServer()

	err := client.AddSshKeyToRepository(ctx, owner, repo1, "My deploy key", "ssh-rsa AAAA...", Read)

	assert.EqualError(t, err, "404 Not Found")
}

func TestBitbucketCloud_GetCommitBySha(t *testing.T) {
	ctx := context.Background()
	sha := "f62ea5359e7af59880b4a5e23e0ce6c1b32b5d3c"
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "commit_single_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/commit/%s", owner, repo1, sha), createBitbucketCloudHandler)
	defer cleanUp()

	result, err := client.GetCommitBySha(ctx, owner, repo1, sha)

	assert.NoError(t, err)
	assert.Equal(t, CommitInfo{
		Hash:          sha,
		AuthorName:    "user",
		CommitterName: "",
		Url:           "https://api.bitbucket.org/2.0/repositories/user2/setup-jfrog-cli/commit/f62ea5359e7af59880b4a5e23e0ce6c1b32b5d3c",
		Timestamp:     1591030449,
		Message:       "Update image name\n",
		ParentHashes:  []string{"f62ea5359e7af59880b4a5e23e0ce6c1b32b5d3c"},
	}, result)
}

func TestBitbucketCloud_GetCommitByShaNotFound(t *testing.T) {
	ctx := context.Background()
	sha := "062ea5359e7af59880b4a5e23e0ce6c1b32b5d3c"
	response := []byte(`<!DOCTYPE html><html lang="en"></html>`)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/commit/%s", owner, repo1, sha),
		http.StatusNotFound,
		createBitbucketCloudHandler)
	defer cleanUp()

	result, err := client.GetCommitBySha(ctx, owner, repo1, sha)
	assert.EqualError(t, err, "404 Not Found")
	assert.Empty(t, result)
}

func createBitbucketCloudWithBodyHandler(t *testing.T, expectedURI string, response []byte, expectedRequestBody []byte,
	expectedStatusCode int, expectedHTTPMethod string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, expectedHTTPMethod, request.Method)
		assert.Equal(t, expectedURI, request.RequestURI)
		assert.Equal(t, basicAuthHeader, request.Header.Get("Authorization"))

		b, err := io.ReadAll(request.Body)
		assert.NoError(t, err)
		assert.Equal(t, expectedRequestBody, b)

		writer.WriteHeader(expectedStatusCode)
		_, err = writer.Write(response)
		assert.NoError(t, err)
	}
}

func TestBitbucketCloud_GetRepositoryInfo(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "repository_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s", owner, repo1), http.StatusOK,
		createBitbucketCloudHandler)
	defer cleanUp()

	res, err := client.GetRepositoryInfo(ctx, owner, repo1)
	assert.NoError(t, err)
	assert.Equal(t,
		RepositoryInfo{
			RepositoryVisibility: Public,
			CloneInfo: CloneInfo{
				HTTP: "https://bitbucket.org/jfrog/jfrog-setup-cli.git",
				SSH:  "git@bitbucket.org:jfrog/jfrog-setup-cli.git",
			},
		},
		res,
	)
}

func TestBitbucketCloud_CreateLabel(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	err = client.CreateLabel(ctx, owner, repo1, LabelInfo{})
	assert.ErrorIs(t, err, errLabelsNotSupported)
}

func TestBitbucketCloud_AddPullRequestReviewComments(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	err = client.AddPullRequestReviewComments(ctx, owner, repo1, 1)
	assert.ErrorIs(t, err, errBitbucketAddPullRequestReviewCommentsNotSupported)
}

func TestBitbucketCloudClient_ListPullRequestReviewComments(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	_, err = client.ListPullRequestReviewComments(ctx, owner, repo1, 1)
	assert.ErrorIs(t, err, errBitbucketListPullRequestReviewCommentsNotSupported)
}

func TestBitbucketCloudClient_DeletePullRequestComment(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	err = client.DeletePullRequestComment(ctx, owner, repo1, 1, 1)
	assert.ErrorIs(t, err, errBitbucketDeletePullRequestComment)
}

func TestBitbucketCloudClient_DeletePullRequestReviewComment(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	err = client.DeletePullRequestReviewComments(ctx, owner, repo1, 1, CommentInfo{})
	assert.ErrorIs(t, err, errBitbucketDeletePullRequestComment)
}

func TestBitbucketCloudClient_DownloadFileFromRepo(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	_, _, err = client.DownloadFileFromRepo(ctx, owner, repo1, branch1, "")
	assert.ErrorIs(t, err, errBitbucketDownloadFileFromRepoNotSupported)
}

func TestBitbucketCloud_GetLabel(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	_, err = client.GetLabel(ctx, owner, repo1, labelName)
	assert.ErrorIs(t, err, errLabelsNotSupported)
}

func TestBitbucketCloud_ListPullRequestLabels(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	_, err = client.ListPullRequestLabels(ctx, owner, repo1, 1)
	assert.ErrorIs(t, err, errLabelsNotSupported)
}

func TestBitbucketCloud_UnlabelPullRequest(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	err = client.UnlabelPullRequest(ctx, owner, repo1, labelName, 1)
	assert.ErrorIs(t, err, errLabelsNotSupported)
}

func TestBitbucketCloud_GetRepositoryEnvironmentInfo(t *testing.T) {
	ctx := context.Background()
	client, err := NewClientBuilder(vcsutils.BitbucketCloud).Build()
	assert.NoError(t, err)

	_, err = client.GetRepositoryEnvironmentInfo(ctx, owner, repo1, envName)
	assert.ErrorIs(t, err, errBitbucketGetRepoEnvironmentInfoNotSupported)
}

func TestBitbucketCloud_getRepositoryVisibility(t *testing.T) {
	assert.Equal(t, Private, getBitbucketCloudRepositoryVisibility(&bitbucket.Repository{Is_private: true}))
	assert.Equal(t, Public, getBitbucketCloudRepositoryVisibility(&bitbucket.Repository{Is_private: false}))
}

func TestBitbucketCloudClient_GetModifiedFiles(t *testing.T) {
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "compare_commits.json"))
		assert.NoError(t, err)

		client, cleanUp := createServerAndClientReturningStatus(t, vcsutils.BitbucketCloud, true, response,
			fmt.Sprintf("/repositories/%s/%s/diffstat/sha-2..sha-1?page=1", owner, repo1), http.StatusOK,
			createBitbucketCloudHandler)
		defer cleanUp()

		res, err := client.GetModifiedFiles(ctx, owner, repo1, "sha-1", "sha-2")
		assert.NoError(t, err)
		assert.Equal(t, []string{"setup.py", "some/full.py"}, res)
	})

	t.Run("validation fails", func(t *testing.T) {
		client := BitbucketCloudClient{}
		_, err := client.GetModifiedFiles(ctx, "", repo1, "sha-1", "sha-2")
		assert.EqualError(t, err, "validation failed: required parameter 'owner' is missing")
		_, err = client.GetModifiedFiles(ctx, owner, "", "sha-1", "sha-2")
		assert.EqualError(t, err, "validation failed: required parameter 'repository' is missing")
		_, err = client.GetModifiedFiles(ctx, owner, repo1, "", "sha-2")
		assert.EqualError(t, err, "validation failed: required parameter 'refBefore' is missing")
		_, err = client.GetModifiedFiles(ctx, owner, repo1, "sha-1", "")
		assert.EqualError(t, err, "validation failed: required parameter 'refAfter' is missing")
	})

	t.Run("failed request", func(t *testing.T) {
		client, cleanUp := createServerAndClientReturningStatus(
			t,
			vcsutils.BitbucketCloud,
			true,
			nil,
			fmt.Sprintf("/repositories/%s/%s/diffstat/sha-2..sha-1?page=1", owner, repo1),
			http.StatusInternalServerError,
			createBitbucketCloudHandler,
		)
		defer cleanUp()
		_, err := client.GetModifiedFiles(ctx, owner, repo1, "sha-1", "sha-2")
		assert.EqualError(t, err, "500 Internal Server Error")
	})
}

func TestBitbucketCloudClient_GetCommitStatus(t *testing.T) {
	ctx := context.Background()
	t.Run("empty response", func(t *testing.T) {
		client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, nil, "/repositories/owner/repo/commit/ref/statuses", createBitbucketCloudHandler)
		defer cleanUp()
		_, err := client.GetCommitStatuses(ctx, "owner", "repo", "ref")
		assert.NoError(t, err)
	})

	t.Run("non empty response", func(t *testing.T) {
		response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "commits_statuses.json"))
		assert.NoError(t, err)
		client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, response, "/repositories/owner/repo/commit/ref/statuses", createBitbucketCloudHandler)
		defer cleanUp()
		commitStatuses, err := client.GetCommitStatuses(ctx, "owner", "repo", "ref")
		assert.NoError(t, err)
		assert.Len(t, commitStatuses, 3)
		assert.Equal(t, InProgress, commitStatuses[0].State)
		assert.Equal(t, Pass, commitStatuses[1].State)
		assert.Equal(t, Fail, commitStatuses[2].State)
	})
}

func TestBitbucketCloud_ListPullRequestReviews(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "pull_request_reviews_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/comments/", owner, repo1, 1), createBitbucketCloudHandler)
	defer cleanUp()

	result, err := client.ListPullRequestReviews(ctx, owner, repo1, 1)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, PullRequestReviewDetails{
		ID:          301545835,
		Reviewer:    "user",
		Body:        "I’m a comment",
		SubmittedAt: "2022-05-16T11:04:07Z",
		CommitID:    "",
	}, result[0])
}

func TestBitbucketCloud_ListPullRequestsAssociatedWithCommit(t *testing.T) {
	ctx := context.Background()
	response, err := os.ReadFile(filepath.Join("testdata", "bitbucketcloud", "pull_requests_associated_with_commit_response.json"))
	assert.NoError(t, err)

	client, cleanUp := createServerAndClient(t, vcsutils.BitbucketCloud, true, response,
		fmt.Sprintf("/repositories/%s/%s/commit/%s/pullrequests", owner, repo1, "commitSHA"), createBitbucketCloudHandler)
	defer cleanUp()

	_, err = client.ListPullRequestsAssociatedWithCommit(ctx, owner, repo1, "commitSHA")
	assert.ErrorIs(t, err, errBitbucketListPullRequestAssociatedCommitsNotSupported)
}

func TestSplitWorkSpaceAndOwner(t *testing.T) {
	valid := "work/repo"
	workspace, repo := splitBitbucketCloudRepoName(valid)
	assert.Equal(t, "work", workspace)
	assert.Equal(t, "repo", repo)

	invalid := "workrepo"
	workspace, repo = splitBitbucketCloudRepoName(invalid)
	assert.Equal(t, "", workspace)
	assert.Equal(t, "", repo)
}

func createBitbucketCloudHandler(t *testing.T, expectedURI string, response []byte, expectedStatusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatusCode)
		if r.RequestURI == "/workspaces" {
			workspacesResults := make(map[string]interface{})
			workspacesResults["values"] = []bitbucket.Workspace{{Slug: username}}
			response, err := json.Marshal(workspacesResults)
			assert.NoError(t, err)
			_, err = w.Write(response)
			assert.NoError(t, err)
		} else {
			_, err := w.Write(response)
			assert.NoError(t, err)
			assert.Equal(t, expectedURI, r.RequestURI)
		}
		assert.Equal(t, basicAuthHeader, r.Header.Get("Authorization"))
	}
}
