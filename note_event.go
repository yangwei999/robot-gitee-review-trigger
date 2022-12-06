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

	return bot.handleCIStatusComment(eventInfo, cfg, log)
}

func (bot *robot) handleReviewComment(e *NoteEventInfo, cfg *botConfig, log *logrus.Entry) error {
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
	rs, rr := info.doStats(stats, bot.botName, e)

	return pa.do(oldTips, cmd, rs, rr, bot.botName)
}

func (bot *robot) isValidReview(
	commandEndpoint string, stats *reviewStats, e *NoteEventInfo, log *logrus.Entry,
) (string, bool) {
	commenter := normalizeLogin(e.GetCommenter())

	cmd, invalidCmd := getReviewCommand(e.cmds.UnsortedList(), commenter, stats.genCheckCmdFunc())

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

//handleCanReviewComment handle the can-review comment send by author
func (bot *robot) handleCanReviewComment(cfg *botConfig, e *NoteEventInfo, log *logrus.Entry) error {
	if !e.isAuthor() {
		return nil
	}
	prInfo := prInfoOnNoteEvent{e.NoteEvent}
	if prInfo.hasLabel(labelCanReview) {
		return nil
	}

	if tip := bot.labelCheckTip(cfg, prInfo); tip != "" {
		org, repo := prInfo.getOrgAndRepo()

		return bot.client.CreatePRComment(
			org, repo, prInfo.getNumber(),
			giteeclient.GenResponseWithReference(e.NoteEvent, tip),
		)
	}

	return bot.readyToReview(prInfo, cfg, log)
}

func (bot *robot) labelCheckTip(cfg *botConfig, prInfo prInfoOnNoteEvent) string {
	commonTip := "You can only comment /can-review when you have signed %s,\n" +
		"You can only comment /can-review when the label of %s is available."
	var tip string
	if !bot.ciCheck(cfg, prInfo) {
		tip = fmt.Sprintf(commonTip, "ci", cfg.CI.LabelForCIPassed)
	}
	if !prInfo.hasLabel(cfg.CLA.LabelForCLAPassed) {
		tip = fmt.Sprintf(commonTip, "cla", cfg.CLA.LabelForCLAPassed)
	}
	return tip
}

func (bot *robot) ciCheck(cfg *botConfig, prInfo prInfoOnNoteEvent) bool {
	return cfg.CI.NoCI || prInfo.hasLabel(cfg.CI.LabelForCIPassed)
}

type NoteEventInfo struct {
	*sdk.NoteEvent
	cmds sets.String
}

func (bot *robot) NewNoteEventInfo(e *sdk.NoteEvent) *NoteEventInfo {
	cmds := parseCommand(e.GetComment().GetBody())

	return &NoteEventInfo{
		e,
		cmds,
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
	return n.GetCommenter() == n.GetPRAuthor()
}
