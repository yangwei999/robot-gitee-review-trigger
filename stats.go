package main

import (
	"sort"
	"time"

	sdk "github.com/opensourceways/go-gitee/gitee"
	"k8s.io/apimachinery/pkg/util/sets"
)

type reviewComment struct {
	author  string
	comment string
	t       time.Time
}

type reviewStats struct {
	pr        *pullRequest
	cfg       reviewConfig
	reviewers sets.String
}

func (rs reviewStats) StatReview(
	comments []sdk.PullRequestComments,
	startTime time.Time,
	botName string,
) (reviewSummary, reviewResult) {

	commands := rs.filterComments(comments, startTime, botName)
	if len(commands) == 0 {
		return reviewSummary{}, reviewResult{}
	}

	r := genReviewSummary(commands)

	return r, genReviewResult(r, rs.pr.unApprovedFiles, rs.cfg)
}

func (rs reviewStats) filterComments(comments []sdk.PullRequestComments, startTime time.Time, botName string) []reviewCommand {
	isValidCmd := rs.genCheckCmdFunc()

	newComments := rs.preTreatComments(comments, startTime, botName)
	n := len(newComments)

	done := map[string]bool{}
	commands := make([]reviewCommand, 0, n)
	for i := n - 1; i >= 0; i-- {
		c := &newComments[i]
		if done[c.author] {
			continue
		}

		if cmd, _ := getReviewCommand(c.comment, c.author, isValidCmd); cmd != "" {
			commands = append(commands, reviewCommand{command: cmd, author: c.author})
			done[c.author] = true
		}
	}

	return commands
}

// first. filter comments and omit each one
// which is before the pr code update time
// or which is not a reviewer
// or which is commented by bot
//
// second sort the comments by updated time in aesc
func (rs reviewStats) preTreatComments(comments []sdk.PullRequestComments, startTime time.Time, botName string) []reviewComment {
	r := make([]reviewComment, 0, len(comments))
	for i := range comments {
		c := &comments[i]

		if c.User == nil || c.User.Login == botName {
			continue
		}

		author := normalizeLogin(c.User.Login)
		if !rs.isReviewer(author) {
			continue
		}

		ut, err := time.Parse(time.RFC3339, c.UpdatedAt)
		if err != nil || ut.Before(startTime) {
			continue
		}

		r = append(r, reviewComment{
			author:  author,
			t:       ut,
			comment: c.Body,
		})
	}

	sort.SliceStable(r, func(i, j int) bool {
		return r[i].t.Before(r[j].t)
	})

	return r
}

func (rs reviewStats) genCheckCmdFunc() func(cmd, author string) bool {
	prAuthor := rs.pr.prAuthor()

	return func(cmd, author string) bool {
		return canApplyCmd(
			cmd,
			prAuthor == author,
			rs.pr.isApprover(author),
			rs.cfg.AllowSelfApprove,
		)
	}
}

func (rs reviewStats) numberOfReviewers() int {
	return len(rs.reviewers)
}

func (rs reviewStats) isReviewer(author string) bool {
	return rs.reviewers.Has(author)
}
