package main

import (
	"fmt"
	"strings"

	"github.com/opensourceways/community-robot-lib/giteeclient"
	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
)

type prInfoOnPREvent struct {
	e *sdk.PullRequestEvent
}

func (pr prInfoOnPREvent) getOrgAndRepo() (string, string) {
	return pr.e.GetOrgRepo()
}

func (pr prInfoOnPREvent) getNumber() int32 {
	return pr.e.GetPRNumber()
}

func (pr prInfoOnPREvent) getTargetBranch() string {
	return pr.e.GetPRBaseRef()
}

func (pr prInfoOnPREvent) hasLabel(l string) bool {
	return pr.e.GetPRLabelSet().Has(l)
}

func (pr prInfoOnPREvent) hasAnyLabel(l []string) bool {
	return pr.e.GetPRLabelSet().HasAny(l...)
}

func (pr prInfoOnPREvent) getAuthor() string {
	return pr.e.GetPRAuthor()
}

func (pr prInfoOnPREvent) getHeadSHA() string {
	return pr.e.GetPRHeadSha()
}

func (pr prInfoOnPREvent) getUrl() string {
	return pr.e.GetPullRequest().GetHtmlURL()
}

func (pr prInfoOnPREvent) getTitle() string {
	return pr.e.GetPullRequest().GetTitle()
}

func (bot *robot) processPREvent(e *sdk.PullRequestEvent, cfg *botConfig, log *logrus.Entry) error {
	switch sdk.GetPullRequestAction(e) {
	case sdk.ActionOpen:
		if cfg.NeedWelcome {
			return bot.welcome(prInfoOnPREvent{e}, cfg)
		}

	case sdk.PRActionChangedSourceBranch:
		return bot.resetToReview(prInfoOnPREvent{e}, cfg, log)

	case sdk.PRActionUpdatedLabel:
		return bot.commentAfterCI(prInfoOnPREvent{e}, cfg)
	}

	return nil
}

func (bot *robot) welcome(pr iPRInfo, cfg *botConfig) error {
	org, repo := pr.getOrgAndRepo()

	s := ""
	if len(cfg.maintainers) > 0 {
		s = fmt.Sprintf(
			"\n\n**%s** will help you to merge this pull-request.\n\n",
			strings.Join(cfg.maintainers, ","),
		)
	}

	doc := ""
	if cfg.doc != "" {
		doc = fmt.Sprintf("\n\n%s.", cfg.doc)
	}

	return bot.client.CreatePRComment(
		org, repo, pr.getNumber(),
		fmt.Sprintf(
			`
Thank you for your pull-request.%s

You can comment **/can-review** to start reviewing when the pr is ready.

The full list of commands accepted by me can be found at [**here**](%s).%s
`,
			s,
			cfg.commandsEndpoint,
			doc,
		),
	)
}

func (bot *robot) readyToReview(pr iPRInfo, cfg *botConfig, log *logrus.Entry) error {
	mr := multiError()

	if err := bot.addLabelOfCanReview(pr); err != nil {
		mr.AddError(err)
	}

	if err := bot.addReviewNotification(pr, cfg, log); err != nil {
		mr.AddError(err)
	}

	return mr.Err()
}

func (bot *robot) addLabelOfCanReview(pr iPRInfo) error {
	l := labelCanReview
	if pr.hasLabel(l) {
		return nil
	}

	org, repo := pr.getOrgAndRepo()
	return bot.client.AddPRLabel(org, repo, pr.getNumber(), l)
}

func (bot *robot) addReviewNotification(pr iPRInfo, cfg *botConfig, log *logrus.Entry) error {
	org, repo := pr.getOrgAndRepo()
	owner, err := bot.genRepoOwner(org, repo, pr.getTargetBranch())
	if err != nil {
		return err
	}

	reviewers, err := suggestReviewers(
		bot.client, owner, pr,
		cfg.Review.TotalNumberOfReviewers,
		cfg.Review.EndpointToRecommendReviewer, log,
	)
	if err != nil {
		return fmt.Errorf("suggest reviewers, err: %s", err.Error())
	}

	if len(reviewers) == 0 {
		return nil
	}

	s := newNotificationComment(&reviewSummary{}, "", bot.botName).startReviewComment(reviewers)

	return bot.client.CreatePRComment(org, repo, pr.getNumber(), s)
}

func (bot *robot) resetToReview(pr iPRInfo, cfg *botConfig, log *logrus.Entry) error {
	mr := multiError()

	if err := bot.resetLabels(pr, cfg); err != nil {
		mr.Add(fmt.Sprintf("remove label when source code changed, err:%s", err.Error()))
	}

	if err := bot.deleteReviewNotification(pr); err != nil {
		mr.Add(fmt.Sprintf("delete tips, err:%s", err.Error()))
	}

	return mr.Err()
}

func (bot *robot) resetLabels(pr iPRInfo, cfg *botConfig) error {
	l := labelUpdating{c: bot.client, pr: pr}
	rmls, err := l.removeLabels([]string{
		labelApproved, labelLGTM, labelRequestChange, labelCanReview,
	})
	if err != nil {
		return err
	}

	if len(rmls) > 0 {
		org, repo := pr.getOrgAndRepo()

		_ = bot.client.CreatePRComment(
			org, repo, pr.getNumber(), fmt.Sprintf(
				"New changes are detected. Remove the following labels: %s.",
				strings.Join(rmls, ", "),
			),
		)
	}

	return nil
}

func (bot *robot) deleteReviewNotification(pr iPRInfo) error {
	org, repo := pr.getOrgAndRepo()

	comments, err := bot.client.ListPRComments(org, repo, pr.getNumber())
	if err != nil {
		return err
	}

	cs := giteeclient.FindBotComment(comments, bot.botName, isNotificationComment)
	for _, c := range cs {
		_ = bot.client.DeletePRComment(org, repo, c.CommentID)
	}

	return nil
}

func (bot *robot) commentAfterCI(pr iPRInfo, cfg *botConfig) error {
	if pr.hasLabel(labelCanReview) {
		return nil
	}

	if !pr.hasAnyLabel(cfg.LabelsForBasicCIPassed) {
		return nil
	}

	org, repo := pr.getOrgAndRepo()
	logs, err := bot.client.ListPROperationLogs(org, repo, pr.getNumber())
	if err != nil {
		return err
	}

	for _, log := range logs {
		for _, label := range cfg.LabelsForBasicCIPassed {
			if !strings.Contains(log.Content, label) {
				continue
			}

			if log.ActionType != sdk.ActionAddLabel {
				return nil
			}

			return bot.client.CreatePRComment(org, repo, pr.getNumber(),
				"You can comment **/can-review** to start reviewing, the pr is ready",
			)
		}
	}

	return nil
}
