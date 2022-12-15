package main

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"sort"

	"github.com/opensourceways/community-robot-lib/utils"
	"github.com/opensourceways/repo-owners-cache/repoowners"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

type RecommendRequest struct {
	Community string   `json:"community"`
	PrUrl     string   `json:"prUrl"`
	PrTitle   string   `json:"prTitle"`
	Reviewers []string `json:"reviewers"`
}

type RecommendResponse struct {
	Msg  string   `json:"msg"`
	Code int      `json:"code"`
	Data []string `json:"data"`
}

func getRecommendReviewers(owner repoowners.RepoOwner, pr iPRInfo, url string) ([]string, error) {
	org, _ := pr.getOrgAndRepo()
	requestData := RecommendRequest{
		Community: org,
		PrUrl:     pr.getUrl(),
		PrTitle:   pr.getTitle(),
		Reviewers: owner.AllReviewers().UnsortedList(),
	}

	payload, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "robot-gitee-review-trigger")

	res := new(RecommendResponse)
	reqCli := utils.HttpClient{
		MaxRetries: 3,
	}
	if err = reqCli.ForwardTo(req, res); err != nil {
		return nil, err
	}

	return res.Data, nil
}

func suggestReviewers(
	c ghclient, owner repoowners.RepoOwner,
	pr iPRInfo, reviewerCount int, log *logrus.Entry,
) ([]string, error) {
	org, repo := pr.getOrgAndRepo()
	changes, err := c.getPullRequestChanges(org, repo, pr.getNumber())
	if err != nil {
		return nil, err
	}

	excludedReviewers := sets.NewString(normalizeLogin(pr.getAuthor()))

	reviewers := getReviewers(owner, changes, reviewerCount, excludedReviewers)
	if len(reviewers) < reviewerCount {

		approvers := getReviewers(
			fallbackReviewersClient{oc: owner},
			changes,
			reviewerCount-len(reviewers),
			excludedReviewers.Insert(reviewers...),
		)
		reviewers = append(reviewers, approvers...)
		sort.Strings(reviewers)

		log.Infof("Added %d approvers as reviewers.", len(approvers))
	}

	if n := len(reviewers); n < reviewerCount {
		log.Warnf(
			"Not enough reviewers found in OWNERS files for files touched by this PR. %d/%d reviewers found.",
			n, reviewerCount,
		)
	}

	return reviewers, nil
}

func getReviewers(rc reviewersClient, files []string, minReviewers int, excludedReviewers sets.String) []string {
	leafReviewers := sets.NewString()
	for _, filename := range files {
		v := rc.LeafReviewers(filename).Difference(excludedReviewers)
		if v.Len() > 0 {
			leafReviewers = leafReviewers.Union(v)
		}
	}

	n := leafReviewers.Len()
	if n == minReviewers {
		return leafReviewers.List()
	}

	if n > minReviewers {
		r := findReviewer(leafReviewers, minReviewers)
		sort.Strings(r)
		return r
	}

	fileReviewers := sets.NewString()
	for _, filename := range files {
		v := rc.Reviewers(filename).Difference(excludedReviewers).Difference(leafReviewers)
		if v.Len() > 0 {
			fileReviewers = fileReviewers.Union(v)
		}
	}

	n = minReviewers - n
	if fileReviewers.Len() <= n {
		return leafReviewers.Union(fileReviewers).List()
	}

	r := findReviewer(fileReviewers, n)
	return leafReviewers.Insert(r...).List()
}

func findReviewer(s sets.String, n int) []string {
	list := s.UnsortedList()
	sort.Strings(list)

	ln := s.Len()
	if ln <= n || n <= 0 {
		return list
	}

	for i := 0; i < n; i++ {
		j := rand.Intn(ln - i)
		k := ln - i - 1
		list[j], list[k] = list[k], list[j]
	}
	return list[ln-n:]
}

type reviewersClient interface {
	Reviewers(path string) sets.String
	LeafReviewers(path string) sets.String
}

type fallbackReviewersClient struct {
	oc repoowners.RepoOwner
}

func (foc fallbackReviewersClient) Reviewers(path string) sets.String {
	return foc.oc.Approvers(path)
}

func (foc fallbackReviewersClient) LeafReviewers(path string) sets.String {
	return foc.oc.LeafApprovers(path)
}
