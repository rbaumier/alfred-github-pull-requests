package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"time"

	aw "github.com/deanishe/awgo"
)

var (
	wf          *aw.Workflow
	githubURL   = "https://api.github.com/graphql"
	searchQuery = `{
		"query": "query {search(query: \"org:%s is:pr is:open\", type: ISSUE, last: 100) {edges {node {... on PullRequest {url title createdAt repository { name }}}}}}"
	}`
)

type Options struct {
	Token        string `env:"GITHUB_TOKEN"`
	Organization string `env:"GITHUB_ORGANIZATION"`
}

type PullRequest struct {
	URL        string    `json:"url"`
	Title      string    `json:"title"`
	CreatedAt  time.Time `json:"createdAt"`
	Repository struct {
		Name string `json:"name"`
	}
}

type Node struct {
	Node PullRequest `json:"node"`
}

type GithubResponse struct {
	Data struct {
		Search struct {
			Edges []Node `json:"edges"`
		} `json:"search"`
	} `json:"data"`
}

func getOptions() (*Options, error) {
	config := aw.NewConfig()
	options := &Options{}
	if err := config.To(options); err != nil {
		return nil, err
	}
	return options, nil
}

func makeRequest(organization, token string) ([]byte, error) {
	gqlQuery := []byte(fmt.Sprintf(searchQuery, organization))
	req, err := http.NewRequest("POST", githubURL, bytes.NewBuffer(gqlQuery))
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	return body, nil
}

func createRepositoriesItems(gr GithubResponse) {
	var pullRequests []PullRequest
	for _, edge := range gr.Data.Search.Edges {
		pullRequests = append(pullRequests, edge.Node)
	}

	sort.Slice(pullRequests, func(i, j int) bool {
		return pullRequests[i].CreatedAt.Before(pullRequests[i].CreatedAt)
	})

	for _, pr := range pullRequests {
		formattedDate := pr.CreatedAt.Format("2006-01-02 15:04")
		wf.NewItem(pr.Title).
			Subtitle(fmt.Sprintf("%s, %s", formattedDate, pr.Repository.Name)).
			Arg(pr.URL).
			Valid(true)
	}
}

func init() {
	wf = aw.New()
}

func run() {
	options, err := getOptions()
	if err != nil {
		wf.FatalError(fmt.Errorf("Cannot get options from environment variables: %v", err))
	}

	body, err := makeRequest(options.Organization, options.Token)
	if err != nil {
		wf.FatalError(fmt.Errorf("Cannot fetch pull requests: %v", err))
	}

	var gr GithubResponse
	err = json.Unmarshal(body, &gr)
	if err != nil {
		wf.FatalError(fmt.Errorf("Cannot parse github response: %v", err))
	}

	createRepositoriesItems(gr)
	wf.WarnEmpty("No Open Pull Requests!", "Good job 😁")
	wf.SendFeedback()
}

func main() {
	wf.Run(run)
}
