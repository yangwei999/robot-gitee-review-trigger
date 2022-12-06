package main

import (
	"fmt"
	"strings"

	"github.com/opensourceways/community-robot-lib/giteeclient"
	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

type NoteEventInfo struct {
	e               *sdk.NoteEvent
	cmds            sets.String
	hasReviewCmd    bool
	hasAuthorCmd    bool
	hasCanReviewCmd bool
	hasAssignCmd    bool
}

func (bot *robot) processNoteEvent(e *sdk.NoteEvent, cfg *botConfig, log *logrus.Entry) error {
	if !e.IsPullRequest() || !e.IsPROpen() {
		return nil
	}

	if e.IsCreatingCommentEvent() && e.GetCommenter() != bot.botName {
		info := bot.NewNoteEventInfo(e)

		mr := multiError()
		if info.hasReviewCmd {
			err := bot.handleReviewComment(e, cfg, log)
			mr.AddError(err)
		}
		if info.hasAuthorCmd {
			err := bot.handleAuthorCommand(info, cfg, log)
			mr.AddError(err)
		}
		return mr.Err()
	}

	return bot.handleCIStatusComment(e, cfg, log)
}

func (bot *robot) NewNoteEventInfo(e *sdk.NoteEvent) *NoteEventInfo {
	cmds := parseCommand(e.GetComment().GetBody())
	info := &NoteEventInfo{
		e:    e,
		cmds: cmds,
	}

	if len(cmds.Intersection(validReviewCmds)) > 0 {
		info.hasReviewCmd = true
	}
	if len(cmds.Intersection(validAuthorCmds)) > 0 {
		info.hasAuthorCmd = true
	}
	if cmds.Has(cmdCanReview) {
		info.hasCanReviewCmd = true
	}

	return info
}

func (bot *robot) handleAuthorCommand(info *NoteEventInfo, cfg *botConfig, log *logrus.Entry) error {
	if info.e.GetCommenter() != info.e.GetPRAuthor() {
		return nil
	}

	mr := multiError()
	if info.hasCanReviewCmd {
		err := bot.handleCanReviewComment(cfg, info.e, log)
		mr.AddError(err)
	}

	return mr.Err()
}

func (bot *robot) handleReviewComment(e *sdk.NoteEvent, cfg *botConfig, log *logrus.Entry) error {
	org, repo := e.GetOrgRepo()
	owner, err := bot.genRepoOwner(org, repo, e.GetPRBaseRef())
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

//handleCanReviewComment handle the can-review comment send by author
func (bot *robot) handleCanReviewComment(cfg *botConfig, e *sdk.NoteEvent, log *logrus.Entry) error {
	prInfo := prInfoOnNoteEvent{e}
	if prInfo.hasLabel(labelCanReview) {
		return nil
	}

	if bot.ciAndClaLabelCheck(cfg, prInfo) {
		return bot.readyToReview(prInfo, cfg, log)
	} else {
		org, repo := prInfo.getOrgAndRepo()

		s := "You can't comment `/can-review` before pass the CI and CLA test"
		return bot.client.CreatePRComment(
			org, repo, prInfo.getNumber(), giteeclient.GenResponseWithReference(e, s),
		)
	}
}

func (bot *robot) ciAndClaLabelCheck(cfg *botConfig, prInfo prInfoOnNoteEvent) bool {
	ciPassed := cfg.CI.NoCI || prInfo.hasLabel(cfg.CI.LabelForCIPassed)
	claPassed := cfg.CLA.NoCLA || prInfo.hasLabel(cfg.CLA.LabelForCLAPassed)
	return ciPassed && claPassed
}
