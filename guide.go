package main

import (
	"fmt"
	"sort"
	"strings"
)

const (
	notificationTitle            = "### Review Guide\n\nThis Pull-Request"
	notificationTitleOld         = "### ~~~ Approval ~~~ Notifier ~~~\nThis Pull-Request"
	notificationSpliter          = "\n#### Tips:\n"
	notificationSpliterOld       = "\n\n---\n\n"
	notificationLineSpliter      = "\n"
	notificationLGTMPart2        = "In order to add **lgtm** label"
	notificationApprovePart2     = "In order to add **approved** label"
	notificationReviewersSpliter = ", "

	reviewStatusStart      = "gets ready to be reviewed"
	reviewStatusInProgress = "is being reviewed"
	reviewStatusRejected   = "is **Rejected**"
	reviewStatusChange     = "is **Requested Change**"
	reviewStatusLGTM       = "is added **lgtm** label"
	reviewStatusApproved   = "is added **approved** label"
	reviewStatusPassReview = "**Passes Review**"
)

func newNotificationComment(rs *reviewSummary, s, botName string) notificationComment {
	return notificationComment{rs: rs, oldTips: s, botName: botName}
}

type notificationComment struct {
	rs      *reviewSummary
	rr      *reviewResult
	oldTips string
	botName string
}

func (n notificationComment) genApproveTips(approvers, ownersFiles []string) string {
	an := ""
	if n.rr.needApproveNum > 0 {
		an = fmt.Sprintf("%d ", n.rr.needApproveNum)
	}

	of := ""
	if len(ownersFiles) > 0 {
		sort.Strings(ownersFiles)

		of = fmt.Sprintf(
			"\nThe relevant OWNERS files are as bellow.\n- %s\n",
			strings.Join(ownersFiles, "\n- "),
		)
	}

	uf := ""
	if len(n.rr.unApprovedFiles) > 0 {
		sort.Strings(n.rr.unApprovedFiles)

		uf = fmt.Sprintf(
			"\nThe unapproved files are as bellow.\n- %s\n",
			strings.Join(n.rr.unApprovedFiles, "\n- "),
		)
	}

	return fmt.Sprintf(
		"%s, it still needs %sapprovers to comment /approve.%s%s\nI suggest these approvers( %s ) to approve your PR.\nYou can assign the PR to them by writing a comment like this `/assign @%s`. Please, replace `%s` with the correct approver's name.",
		notificationApprovePart2,
		an, uf, of,
		toReviewerList(approvers),
		n.botName,
		n.botName,
	)
}

func (n notificationComment) genLGTMTips(num int, suggestedReviewers []string) string {
	key := "reviewers to comment /lgtm."
	p1 := fmt.Sprintf("%s, it still needs **%d** %s", notificationLGTMPart2, num, key)

	if len(suggestedReviewers) > 0 {
		s2 := fmt.Sprintf(
			"\nI suggest these reviewers( %s ) to review your codes.\nYou can ask them to review by writing a comment like this `@%s, Could you take a look at this PR, thanks!`. Please, replace `%s` with correct reviewer's name",
			toReviewerList(suggestedReviewers),
			n.botName,
			n.botName,
		)

		return n.genPart2(p1 + s2)
	}

	if !containsSuggestedReviewer(n.oldTips) || !strings.Contains(n.oldTips, key) {
		return ""
	}

	if v := strings.Split(n.oldTips, key); len(v) > 1 {
		return n.genPart2(p1 + v[1])
	}

	return ""

}

func (n notificationComment) startReviewComment(reviewers []string) string {
	s := n.genLGTMTips(len(reviewers), reviewers)
	return fmt.Sprintf("%s %s.%s", notificationTitle, reviewStatusStart, s)
}

func (n notificationComment) reviewingComment(num int, reviewers []string) string {
	tips := ""
	if v := n.rs.disagreedReviewers; len(v) > 0 {
		tips = fmt.Sprintf(
			"%sReviewers who writed a comment of `/lbtm` are: %s. Please make changes if it needs.",
			notificationLGTMPart2, toReviewerList(v),
		)
	}

	if s := n.reviewInfo(); s != "" {
		tips += notificationLineSpliter + s
	}

	p2 := n.genLGTMTips(num, reviewers)

	return fmt.Sprintf("%s %s.%s%s", notificationTitle, reviewStatusInProgress, tips, p2)
}

func (n notificationComment) rejectComment() string {
	return fmt.Sprintf(
		"%s %s.%sIt is rejected by: %s. Please see the comments left by them and do more changes.",
		notificationTitle,
		reviewStatusRejected,
		notificationLineSpliter,
		toReviewerList(n.rs.disagreedApprovers),
	)
}

func (n notificationComment) requestChangeComment() string {
	return fmt.Sprintf(
		"%s %s.%sIt is requested change by: %s. Please see the comments left by them and do more changes.",
		notificationTitle,
		reviewStatusChange,
		notificationLineSpliter,
		toReviewerList(n.rs.disagreedReviewers),
	)
}

func (n notificationComment) passReviewComment() string {
	s := n.reviewInfo()
	if s != "" {
		s = notificationLineSpliter + s
	}

	return fmt.Sprintf("%s %s.%s", notificationTitle, reviewStatusPassReview, s)
}

func (n notificationComment) approvedComment(num int, suggestedReviewers []string) string {
	s := n.reviewInfo()
	if s != "" {
		s = notificationLineSpliter + s
	}

	s1 := n.genLGTMTips(num, suggestedReviewers)

	return fmt.Sprintf(
		"%s %s. In order to pass review, it still needs **lgtm** label.%s%s",
		notificationTitle, reviewStatusApproved, s, s1,
	)
}

func (n notificationComment) lgtmComment(suggestedApprovers, ownersFiles []string) string {
	s := n.reviewInfo()
	if s != "" {
		s = notificationLineSpliter + s
	}

	s1 := n.getPart2OfApproved(suggestedApprovers, ownersFiles)

	return fmt.Sprintf(
		"%s %s. In order to pass review, it still needs **approved** label.%s%s",
		notificationTitle, reviewStatusLGTM, s, s1,
	)
}

func (n notificationComment) reviewInfo() string {
	rs := n.rs
	s := ""
	if len(rs.agreedApprovers) > 0 {
		s = fmt.Sprintf(
			"Approvers who writed a comment of `/approve` are: %s.",
			toReviewerList(rs.agreedApprovers),
		)
	}

	s1 := ""
	if len(rs.agreedReviewers) > 0 {
		s1 = fmt.Sprintf(
			"Reviewers who writed a comment of `/lgtm` are: %s.",
			toReviewerList(rs.agreedReviewers),
		)
	}

	if s != "" && s1 != "" {
		return s + notificationLineSpliter + s1
	}
	return s + s1
}

func (n notificationComment) getPart2OfApproved(suggestedApprovers, ownersFiles []string) string {
	if len(suggestedApprovers) > 0 {
		return n.genPart2(n.genApproveTips(suggestedApprovers, ownersFiles))
	}

	if !containsSuggestedApprover(n.oldTips) {
		return ""
	}

	if v := strings.Split(n.oldTips, notificationSpliter); len(v) == 2 {
		return n.genPart2(v[1])
	}

	if v := strings.Split(n.oldTips, notificationSpliterOld); len(v) == 2 {
		return n.genPart2(v[1])
	}
	return ""
}

func (n notificationComment) genPart2(c string) string {
	if c != "" {
		return notificationSpliter + c
	}
	return c
}

func convertReviewers(v []string) []string {
	rs := make([]string, 0, len(v))
	for _, item := range v {
		rs = append(rs, fmt.Sprintf("[*%s*](https://gitee.com/%s)", item, item))
	}
	return rs
}

func toReviewerList(v []string) string {
	return strings.Join(convertReviewers(v), notificationReviewersSpliter)
}

func containsSuggestedApprover(c string) bool {
	return strings.Contains(c, notificationApprovePart2)
}

func containsSuggestedReviewer(c string) bool {
	return strings.Contains(c, notificationLGTMPart2)
}

func isNotificationComment(c string) bool {
	return strings.HasPrefix(c, notificationTitle) || strings.HasPrefix(c, notificationTitleOld)
}
