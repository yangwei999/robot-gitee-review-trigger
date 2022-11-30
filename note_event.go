package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/opensourceways/community-robot-lib/giteeclient"
	sdk "github.com/opensourceways/go-gitee/gitee"
)

var assignRe = regexp.MustCompile(`(?mi)^/(un)?assign(( @?[-\w]+?)*)\s*$`)

func (bot *robot) processNoteEvent(e *sdk.NoteEvent, cfg *botConfig, log *logrus.Entry) error {
	if !e.IsPullRequest() || !e.IsPROpen() {
		return nil
	}

	if e.IsCreatingCommentEvent() && e.GetCommenter() != bot.botName {
		mr := multiError()
		if cmds := parseReviewCommand(e.GetComment().GetBody()); len(cmds) > 0 {
			err := bot.handleReviewComment(e, cfg, log)
			mr.AddError(err)
		}
		if cmds := parseAuthorCommand(e.GetComment().GetBody()); len(cmds) > 0 {
			err := bot.handleAuthorCommand(e, cfg, cmds, log)
			mr.AddError(err)
		}
		return mr.Err()
	}

	return bot.handleCIStatusComment(e, cfg, log)
}

func (bot *robot) handleAuthorCommand(e *sdk.NoteEvent, cfg *botConfig, cmds []string, log *logrus.Entry) error {
	if e.GetCommenter() != e.GetPRAuthor() {
		return nil
	}
	mr := multiError()

	if sets.NewString(cmds...).HasAny([]string{cmdASSIGN, cmdUNASSIGN}...) {
		err := bot.handleAssignComment(cfg, e)
		mr.AddError(err)
	}

	return mr.Err()
}

func (bot *robot) handleReviewComment(e *sdk.NoteEvent, cfg *botConfig, log *logrus.Entry) error {
	org, repo := e.GetOrgRepo()
	owner, err := bot.genRepoOwner(org, repo, e.GetPRBaseRef(), cfg.Owner, log)
	if err != nil {
		return err
	}

	prInfo := prInfoOnNoteEvent{e}
	pr, err := bot.genPullRequest(prInfo, getAssignees(e.GetPullRequest()), owner)
	if err != nil {
		return err
	}

	stats := &reviewStats{
		pr:        &pr,
		cfg:       cfg.Review,
		reviewers: owner.AllReviewers(),
	}

	cmd, validReview := bot.isValidReview(cfg.commandsEndpoint, stats, e, log)
	if !validReview {
		return nil
	}

	info, err := bot.getReviewInfo(prInfo)
	if err != nil {
		return err
	}

	canReview := cfg.CI.NoCI || stats.pr.info.hasLabel(cfg.CI.LabelForCIPassed)
	pa := PostAction{
		c:                bot.client,
		cfg:              cfg,
		owner:            owner,
		log:              log,
		pr:               &pr,
		isStartingReview: canReview,
	}

	oldTips := info.reviewGuides(bot.botName)
	rs, rr := info.doStats(stats, bot.botName)

	return pa.do(oldTips, cmd, rs, rr, bot.botName)
}

func (bot *robot) isValidReview(
	commandEndpoint string, stats *reviewStats, e *sdk.NoteEvent, log *logrus.Entry,
) (string, bool) {
	commenter := normalizeLogin(e.GetCommenter())

	cmd, invalidCmd := getReviewCommand(e.GetComment().GetBody(), commenter, stats.genCheckCmdFunc())

	validReview := cmd != "" && stats.isReviewer(commenter)

	if !validReview {
		log.Infof(
			"It can't handle note event, because cmd(%s) is empty or commenter(%s) is not a reviewer. There are %d reviewers.",
			cmd, commenter, stats.numberOfReviewers(),
		)
	}

	if invalidCmd != "" {

		info := stats.pr.info
		org, repo := info.getOrgAndRepo()

		s := fmt.Sprintf(
			"You can't comment `/%s`. Please see the [*Command Usage*](%s) to get detail.",
			strings.ToLower(invalidCmd),
			commandEndpoint,
		)

		bot.client.CreatePRComment(
			org, repo, info.getNumber(),
			giteeclient.GenResponseWithReference(e, s),
		)
	}

	return cmd, validReview
}

// handleAssignComment handle the assign comment send by author
func (bot *robot) handleAssignComment(cfg *botConfig, e *sdk.NoteEvent) error {
	pr := prInfoOnNoteEvent{e}
	org, repo := e.GetOrgRepo()
	assign, unassign := bot.parseCmd(e)

	mr := multiError()
	if len(assign) > 0 {
		emailContent := fmt.Sprintf("%s invites you to be a assignee of PR called %s in %s/%s, the PR url is:\n%s",
			pr.getAuthor(), pr.getTitle(), org, repo, pr.getUrl())
		err := NewEmailService(cfg).SendEmailToReviewers(assign.UnsortedList(), emailContent)
		mr.AddError(err)

	}
	if len(unassign) > 0 {
		emailContent := fmt.Sprintf("you have been unassigned as a assignee of PR called %s in %s/%s, the PR url is:\n%s",
			pr.getTitle(), org, repo, pr.getUrl())
		err := NewEmailService(cfg).SendEmailToReviewers(unassign.UnsortedList(), emailContent)
		mr.AddError(err)

	}

	return mr.Err()
}

func (bot *robot) parseCmd(e *sdk.NoteEvent) (sets.String, sets.String) {
	assign := sets.NewString()
	unassign := sets.NewString()

	f := func(action string, v ...string) {
		if action == "" {
			assign.Insert(v...)
		} else {
			unassign.Insert(v...)
		}
	}

	matches := assignRe.FindAllStringSubmatch(e.Comment.Body, -1)
	for _, re := range matches {
		if re[2] == "" {
			f(re[1], e.GetCommenter())
		} else {
			f(re[1], bot.parseLogins(re[2])...)
		}
	}

	return assign, unassign
}

func (bot *robot) parseLogins(text string) []string {
	var parts []string
	for _, s := range strings.Split(text, " ") {
		if v := strings.Trim(s, "@ "); v != "" {
			parts = append(parts, v)
		}
	}
	return parts
}
