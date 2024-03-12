package main

import (
	"fmt"
	"strings"

	"github.com/opensourceways/community-robot-lib/giteeclient"
	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

func (bot *robot) processNoteEvent(e *sdk.NoteEvent, cfg *botConfig, log *logrus.Entry) error {
	if !e.IsPullRequest() || !e.IsPROpen() {
		return nil
	}

	if !e.IsCreatingCommentEvent() || e.GetCommenter() == bot.botName {
		return nil
	}

	cmds := parseCommentCommands(e.GetComment().GetBody())

	info := &noteEventInfo{
		NoteEvent: e,
		cmds:      sets.NewString(cmds...),
	}

	mr := multiError()

	if info.hasCheckPRCmd() && cfg.CheckPRInfo {
		text := e.GetPullRequest().GetBody()
		org, repo := e.GetOrgRepo()
		author := e.GetPRAuthor()
		number := e.GetPRNumber()
		err := bot.setTestNumber(text, org, repo, author, number)
		mr.AddError(err)
	}

	if info.hasReviewCmd() {
		err := bot.handleReviewComment(info, cfg, log)
		mr.AddError(err)
	}

	if info.hasCanReviewCmd() {
		err := bot.handleCanReviewComment(info, cfg, log)
		mr.AddError(err)
	}

	return mr.Err()
}

func (bot *robot) handleReviewComment(e *noteEventInfo, cfg *botConfig, log *logrus.Entry) error {
	org, repo := e.GetOrgRepo()
	owner, err := bot.genRepoOwner(org, repo, e.GetPRBaseRef())
	if err != nil {
		return err
	}

	prInfo := prInfoOnNoteEvent{e.NoteEvent}
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

	pa := PostAction{
		c:                bot.client,
		cfg:              cfg,
		owner:            owner,
		log:              log,
		pr:               &pr,
		isStartingReview: stats.pr.info.hasLabel(labelCanReview),
	}

	oldTips := info.reviewGuides(bot.botName)
	rs, rr := info.doStats(stats, bot.botName)

	return pa.do(oldTips, cmd, rs, rr, bot.botName)
}

func (bot *robot) isValidReview(
	commandEndpoint string, stats *reviewStats, e *noteEventInfo, log *logrus.Entry,
) (string, bool) {
	cmd, invalidCmd := e.checkReviewCmd(stats.genCheckCmdFunc())

	commenter := e.normalizedCommenter()
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
			giteeclient.GenResponseWithReference(e.NoteEvent, s),
		)
	}

	return cmd, validReview
}

type noteEventInfo struct {
	*sdk.NoteEvent
	cmds sets.String
}

func (n *noteEventInfo) getReviewCmd() []string {
	return n.cmds.Intersection(validCmds).UnsortedList()
}

func (n *noteEventInfo) hasReviewCmd() bool {
	return len(n.getReviewCmd()) > 0
}

func (n *noteEventInfo) hasCanReviewCmd() bool {
	return n.cmds.Has(cmdCanReview)
}

func (n *noteEventInfo) hasCheckPRCmd() bool {
	return n.cmds.Has(cmdCheckPR)
}

func (n *noteEventInfo) isCommentedByPRAuthor() bool {
	return n.GetCommenter() == n.GetPRAuthor()
}

func (n *noteEventInfo) normalizedCommenter() string {
	return normalizeLogin(n.GetCommenter())
}

func (n *noteEventInfo) checkReviewCmd(isValidCmd func(cmd, author string) bool) (
	string, string,
) {
	author := n.normalizedCommenter()

	return checkReviewCommand(n.getReviewCmd(), func(cmd string) bool {
		return isValidCmd(cmd, author)
	})
}
