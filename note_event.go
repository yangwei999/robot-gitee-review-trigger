package main

import (
	"fmt"
	"strings"

	"github.com/opensourceways/community-robot-lib/giteeclient"
	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
)

func (bot *robot) processNoteEvent(e *sdk.NoteEvent, cfg *botConfig, log *logrus.Entry) error {
	if !e.IsPullRequest() || !e.IsPROpen() {
		return nil
	}

	if e.IsCreatingCommentEvent() && e.GetCommenter() != bot.botName {
		info := newCommentInfo(e.Comment.Body, e.GetCommenter())

		mr := multiError()
		if info.hasReviewCmd() {
			err := bot.handleReviewComment(e, cfg, info, log)
			mr.AddError(err)
		}

		if info.hasCanReviewCmd() {
			err := bot.handleCanReviewComment(cfg, e, log)
			mr.AddError(err)
		}

		return mr.Err()
	}

	return bot.handleCIStatusComment(e, cfg, log)
}

func (bot *robot) handleReviewComment(e *sdk.NoteEvent, cfg *botConfig, cInfo *commentInfo, log *logrus.Entry) error {
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

	cmd, validReview := bot.isValidReview(cfg.commandsEndpoint, stats, e, cInfo, log)
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
	commandEndpoint string,
	stats *reviewStats,
	e *sdk.NoteEvent,
	cInfo *commentInfo,
	log *logrus.Entry,
) (string, bool) {
	commenter := normalizeLogin(e.GetCommenter())

	cmd, invalidCmd := cInfo.validateReviewCmd(stats.genCheckCmdFunc())
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
	if e.GetCommenter() != e.GetPRAuthor() {
		return nil
	}

	prInfo := prInfoOnNoteEvent{e}
	if prInfo.hasLabel(labelCanReview) {
		return nil
	}

	if tip := bot.labelCheckTip(cfg, prInfo); tip != "" {
		org, repo := prInfo.getOrgAndRepo()

		return bot.client.CreatePRComment(
			org, repo, prInfo.getNumber(),
			giteeclient.GenResponseWithReference(e, tip),
		)
	}

	return bot.readyToReview(prInfo, cfg, log)
}

func (bot *robot) labelCheckTip(cfg *botConfig, prInfo prInfoOnNoteEvent) string {
	if !bot.ciCheck(cfg, prInfo) {
		tip := "You can only comment /can-review when the label of %s is available."
		return fmt.Sprintf(tip, cfg.CI.LabelForCIPassed)
	}

	if !prInfo.hasLabel(cfg.CLALabel) {
		return "You can only comment /can-review when you have signed cla"
	}

	return ""
}

func (bot *robot) ciCheck(cfg *botConfig, prInfo prInfoOnNoteEvent) bool {
	return cfg.CI.NoCI || prInfo.hasLabel(cfg.CI.LabelForCIPassed)
}
