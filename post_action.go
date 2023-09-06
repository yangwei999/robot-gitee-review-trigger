package main

import (
	"github.com/opensourceways/community-robot-lib/giteeclient"
	"github.com/opensourceways/repo-owners-cache/repoowners"
	"github.com/sirupsen/logrus"
)

type PostAction struct {
	cfg   *botConfig
	log   *logrus.Entry
	pr    *pullRequest
	c     ghclient
	owner repoowners.RepoOwner

	isStartingReview bool
}

type actionParameter struct {
	deleteOldComments func()
	writeNotification func(string) error
	oldTips           string
	lastComment       string
	n                 notificationComment
	u                 func(...string) error
}

func (pa PostAction) do(oldComments []giteeclient.BotComment, lastComment string, rs reviewSummary, r reviewResult, botName string) error {
	if rs.IsEmpty() {
		return nil
	}

	oldTips := ""
	if i := len(oldComments); i > 0 {
		if i > 1 {
			giteeclient.SortBotComments(oldComments)
		}
		oldTips = oldComments[i-1].Body
	}

	deleteOldComments := func() {
		org, repo := pa.pr.info.getOrgAndRepo()

		for _, c := range oldComments {
			pa.c.DeletePRComment(org, repo, c.CommentID)
		}
	}

	param := &actionParameter{
		lastComment:       lastComment,
		deleteOldComments: deleteOldComments,

		n: notificationComment{
			rs:      &rs,
			rr:      &r,
			oldTips: oldTips,
			botName: botName,
		},

		u: func(keep ...string) error {
			return updatePRLabel(pa.c, pa.pr.info, keep...)
		},
	}

	param.writeNotification = func(desc string) error {
		if desc == oldTips {
			return nil
		}

		var err error
		if desc != "" {
			info := pa.pr.info
			org, repo := info.getOrgAndRepo()

			err = pa.c.CreatePRComment(org, repo, info.getNumber(), desc)
		}

		deleteOldComments()

		return err
	}

	return pa.handle(param, &r)
}

func (pa PostAction) handle(param *actionParameter, r *reviewResult) error {
	if r.isRejected {
		return pa.reject(param)
	}

	if r.isLBTM {
		return pa.requestChange(param)
	}

	if r.isLGTM && r.isApproved {
		return pa.passReview(param)
	}

	if r.isLGTM {
		return pa.lgtm(param, r)
	}

	if r.isApproved {
		return pa.approve(param, r)
	}

	return pa.reviewing(param, r)
}

func (pa PostAction) reviewing(p *actionParameter, r *reviewResult) error {
	if !pa.isStartingReview {
		p.deleteOldComments()
		return p.u()
	}

	mr := multiError()

	if err := p.u(labelCanReview); err != nil {
		mr.AddError(err)
	}

	var sr []string
	if p.oldTips == "" || !containsSuggestedReviewer(p.oldTips) {
		sr = pa.suggestReviewers()
	}

	s := p.n.reviewingComment(r.needLGTMNum, sr)
	if err := p.writeNotification(s); err != nil {
		mr.AddError(err)
	}

	return mr.Err()
}

func (pa PostAction) reject(p *actionParameter) error {
	mr := multiError()

	if err := p.u(labelRequestChange); err != nil {
		mr.AddError(err)
	}

	s := p.n.rejectComment()
	if err := p.writeNotification(s); err != nil {
		mr.AddError(err)
	}

	return mr.Err()
}

func (pa PostAction) requestChange(p *actionParameter) error {
	mr := multiError()

	if err := p.u(labelRequestChange); err != nil {
		mr.AddError(err)
	}

	c := p.n.requestChangeComment()
	if err := p.writeNotification(c); err != nil {
		mr.AddError(err)
	}

	return mr.Err()
}

func (pa PostAction) lgtm(p *actionParameter, r *reviewResult) error {
	mr := multiError()

	if err := p.u(labelLGTM); err != nil {
		mr.AddError(err)
	}

	if !pa.isStartingReview {
		p.deleteOldComments()
		return mr.Err()
	}

	oldTips := p.oldTips
	lastComment := p.lastComment
	needSuggestApprover := oldTips == "" || !containsSuggestedApprover(oldTips) || lastComment == cmdAPPROVE

	s := ""
	if needSuggestApprover {
		sa := pa.suggestApprovers(p.n.rs.agreedApprovers)
		ownersFiles := pa.relevantOwnersFiles(r.unApprovedFiles)

		s = p.n.lgtmComment(sa, ownersFiles)
	} else {
		s = p.n.lgtmComment(nil, nil)
	}

	if err := p.writeNotification(s); err != nil {
		mr.AddError(err)
	}

	return mr.Err()
}

func (pa PostAction) approve(p *actionParameter, r *reviewResult) error {
	mr := multiError()

	if err := p.u(labelApproved); err != nil {
		mr.AddError(err)
	}

	if !pa.isStartingReview {
		p.deleteOldComments()
		return mr.Err()
	}

	oldTips := p.oldTips
	needSuggestReviewer := oldTips == "" || !containsSuggestedReviewer(oldTips)

	var sr []string
	if needSuggestReviewer {
		sr = pa.suggestReviewers()
	}

	s := p.n.approvedComment(r.needLGTMNum, sr)
	if err := p.writeNotification(s); err != nil {
		mr.AddError(err)
	}
	return mr.Err()
}

func (pa PostAction) passReview(p *actionParameter) error {
	mr := multiError()

	if err := p.u(labelLGTM, labelApproved); err != nil {
		mr.AddError(err)
	}

	c := p.n.passReviewComment()
	if err := p.writeNotification(c); err != nil {
		mr.AddError(err)
	}

	return mr.Err()
}

func (pa PostAction) suggestApprovers(currentApprovers []string) []string {
	return suggestingApprover{
		pr:    pa.pr,
		cfg:   pa.cfg.Review,
		owner: pa.owner,
	}.suggestApprover(
		currentApprovers, pa.pr.assignees, pa.log,
	)
}

func (pa PostAction) relevantOwnersFiles(files []string) []string {
	if len(files) == 0 {
		return nil
	}

	m := make(map[string]bool)
	for _, f := range files {
		if v := pa.owner.FindApproverOwnersForFile(f); !m[v] {
			m[v] = true
		}
	}

	i := 0
	r := make([]string, len(m))
	for k := range m {
		r[i] = k
		i++
	}

	return r
}

func (pa PostAction) suggestReviewers() []string {
	v, err := suggestReviewers(
		pa.c, pa.owner, pa.pr.info,
		pa.cfg.Review.TotalNumberOfReviewers,
		pa.cfg.Review.EndpointToRecommendReviewer,
		pa.log,
	)
	if err != nil {
		pa.log.Error(err)
	}
	return v
}
