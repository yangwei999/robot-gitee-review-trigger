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

	eventInfo := bot.NewNoteEventInfo(e)
	if e.IsCreatingCommentEvent() && e.GetCommenter() != bot.botName {

		mr := multiError()
		if eventInfo.hasReviewCmd() {
			err := bot.handleReviewComment(cfg, eventInfo, log)
			mr.AddError(err)
		}
		if eventInfo.hasCanReviewCmd() {
			err := bot.handleCanReviewComment(cfg, eventInfo, log)
			mr.AddError(err)
		}
		return mr.Err()
	}

	return bot.handleCIStatusComment(cfg, eventInfo, log)
}

func (bot *robot) handleReviewComment(cfg *botConfig, eventInfo *NoteEventInfo, log *logrus.Entry) error {
	org, repo := eventInfo.e.GetOrgRepo()
	owner, err := bot.genRepoOwner(org, repo, eventInfo.e.GetPRBaseRef())
	if err != nil {
		return err
	}

	prInfo := prInfoOnNoteEvent{eventInfo.e}
	pr, err := bot.genPullRequest(prInfo, getAssignees(eventInfo.e.GetPullRequest()), owner)
	if err != nil {
		return err
	}

	stats := &reviewStats{
		pr:        &pr,
		cfg:       cfg.Review,
		reviewers: owner.AllReviewers(),
	}

	cmd, validReview := bot.isValidReview(cfg.commandsEndpoint, stats, eventInfo, log)
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
	rs, rr := info.doStats(stats, bot.botName, eventInfo)

	return pa.do(oldTips, cmd, rs, rr, bot.botName)
}

func (bot *robot) isValidReview(
	commandEndpoint string, stats *reviewStats, eventInfo *NoteEventInfo, log *logrus.Entry,
) (string, bool) {
	commenter := normalizeLogin(eventInfo.e.GetCommenter())

	cmd, invalidCmd := getReviewCommand(commenter, eventInfo.cmds.UnsortedList(), stats.genCheckCmdFunc())

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
			giteeclient.GenResponseWithReference(eventInfo.e, s),
		)
	}

	return cmd, validReview
}

//handleCanReviewComment handle the can-review comment send by author
func (bot *robot) handleCanReviewComment(cfg *botConfig, eventInfo *NoteEventInfo, log *logrus.Entry) error {
	if !eventInfo.isAuthor() {
		return nil
	}
	prInfo := prInfoOnNoteEvent{eventInfo.e}
	if prInfo.hasLabel(labelCanReview) {
		return nil
	}

	if tip := bot.ciAndClaLabelCheck(cfg, prInfo); tip != "" {
		org, repo := prInfo.getOrgAndRepo()

		return bot.client.CreatePRComment(
			org, repo, prInfo.getNumber(),
			giteeclient.GenResponseWithReference(eventInfo.e, tip),
		)
	}

	return bot.readyToReview(prInfo, cfg, log)
}

func (bot *robot) ciAndClaLabelCheck(cfg *botConfig, prInfo prInfoOnNoteEvent) string {
	ciPassed := cfg.CI.NoCI || prInfo.hasLabel(cfg.CI.LabelForCIPassed)
	claPassed := prInfo.hasLabel(cfg.CLA.LabelForCLAPassed)

	commonTip := "You can only comment /can-review when the label of ci and cla available,\nit still needs %s"
	var tip string
	if !ciPassed {
		tip = fmt.Sprintf(commonTip, cfg.CI.LabelForCIPassed)
	}
	if !claPassed {
		tip = fmt.Sprintf(commonTip, cfg.CLA.LabelForCLAPassed)
	}
	if !ciPassed && !claPassed {
		bothNeedTip := fmt.Sprintf("%s and %s", cfg.CI.LabelForCIPassed, cfg.CLA.LabelForCLAPassed)
		tip = fmt.Sprintf(commonTip, bothNeedTip)
	}

	return tip
}

type NoteEventInfo struct {
	e    *sdk.NoteEvent
	cmds sets.String
}

func (bot *robot) NewNoteEventInfo(e *sdk.NoteEvent) *NoteEventInfo {
	cmds := parseCommand(e.GetComment().GetBody())

	return &NoteEventInfo{
		e:    e,
		cmds: cmds,
	}
}

func (n *NoteEventInfo) hasReviewCmd() bool {
	return len(n.cmds.Intersection(validReviewCmds)) > 0
}

func (n *NoteEventInfo) hasCanReviewCmd() bool {
	return n.cmds.Has(cmdCanReview)
}

func (n *NoteEventInfo) hasAssignCmd() bool {
	return false
}

func (n *NoteEventInfo) isAuthor() bool {
	return n.e.GetCommenter() == n.e.GetPRAuthor()
}
