package main

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/sets"
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
		if cmds := parseReviewCommand(e.GetComment().GetBody()); len(cmds) > 0 {
			return bot.handleReviewComment(e, cfg, cmds, log)
		}
	}

	return bot.handleCIStatusComment(e, cfg, log)
}

func (bot *robot) handleReviewComment(e *sdk.NoteEvent, cfg *botConfig, cmds []string, log *logrus.Entry) error {
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

	if sets.NewString(cmds...).Has(cmdCanReview) && (e.GetCommenter() == e.GetPRAuthor()) {
		return bot.handleCanReviewComment(pr, cfg, log)
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
func (bot *robot) handleCanReviewComment(pr pullRequest, cfg *botConfig, log *logrus.Entry) error {
	for lb, _ := range validCmds {
		if pr.info.hasLabel(lb) {
			return nil
		}
	}

	if pr.info.hasLabel(cfg.CI.LabelForCIPassed) {
		return bot.readyToReview(pr.info, cfg, log)
	} else {
		org, repo := pr.info.getOrgAndRepo()

		return bot.client.CreatePRComment(
			org, repo, pr.info.getNumber(), "it needs to pass the CI and CLA test",
		)
	}
}
