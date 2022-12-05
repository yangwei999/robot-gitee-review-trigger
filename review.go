package main

import (
	"time"

	"github.com/opensourceways/community-robot-lib/giteeclient"
	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/opensourceways/repo-owners-cache/repoowners"
)

func (bot *robot) genRepoOwner(org, repo, branch string) (repoowners.RepoOwner, error) {
	owners, err := repoowners.NewRepoOwners(
		repoowners.RepoBranch{
			Platform: "gitee",
			Org:      org,
			Repo:     repo,
			Branch:   branch,
		},
		bot.cacheCli,
	)
	if err != nil {
		return nil, err
	}
	if owners != nil {
		return owners, nil
	}

	cs, err := bot.client.listCollaborators(org, repo)
	if err != nil {
		return nil, err
	}
	return repoowners.RepoMemberAsOwners(cs), nil
}

func (bot *robot) genPullRequest(prInfo iPRInfo, assignees []string, owner repoowners.RepoOwner) (pullRequest, error) {
	org, repo := prInfo.getOrgAndRepo()
	filenames, err := bot.client.getPullRequestChanges(org, repo, prInfo.getNumber())
	if err != nil {
		return pullRequest{}, err
	}

	return newPullRequest(prInfo, filenames, assignees, owner), nil
}

func (bot *robot) getReviewInfo(info iPRInfo) (ri reviewInfo, err error) {
	org, repo := info.getOrgAndRepo()

	ri.comments, err = bot.client.ListPRComments(org, repo, info.getNumber())
	if err != nil {
		return
	}

	ri.t, err = bot.client.getPRCodeUpdateTime(org, repo, info.getHeadSHA())
	return
}

type reviewInfo struct {
	comments []sdk.PullRequestComments
	t        time.Time
}

func (r reviewInfo) reviewGuides(botName string) []giteeclient.BotComment {
	return giteeclient.FindBotComment(r.comments, botName, isNotificationComment)
}

func (r reviewInfo) doStats(s *reviewStats, botName string) (reviewSummary, reviewResult) {
	return s.StatReview(r.comments, r.t, botName)
}
