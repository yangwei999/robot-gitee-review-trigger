package main

import (
	"regexp"
	"strings"

	"github.com/opensourceways/community-robot-lib/utils"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	labelCanReview     = "can-review"
	labelLGTM          = "lgtm"
	labelApproved      = "approved"
	labelRequestChange = "request-change"

	cmdCanReview = "CAN-REVIEW"
	cmdLGTM      = "LGTM"
	cmdLBTM      = "LBTM"
	cmdAPPROVE   = "APPROVE"
	cmdReject    = "REJECT"
)

var (
	reviewCmds           = sets.NewString(cmdLGTM, cmdLBTM, cmdAPPROVE, cmdReject)
	authorCmds           = sets.NewString(cmdCanReview)
	negativeCmds         = sets.NewString(cmdReject, cmdLBTM)
	positiveCmds         = sets.NewString(cmdAPPROVE, cmdLGTM)
	cmdBelongsToApprover = sets.NewString(cmdAPPROVE, cmdReject)
	commandRegex         = regexp.MustCompile(`(?m)^/([^\s]+)[\t ]*([^\n\r]*)`)
)

func canApplyCmd(cmd string, isPRAuthor, isApprover, allowSelfApprove bool) bool {
	switch cmd {
	case cmdReject:
		return isApprover && !isPRAuthor
	case cmdLGTM:
		return !isPRAuthor
	case cmdAPPROVE:
		return isApprover && (allowSelfApprove || !isPRAuthor)
	}
	return true
}

type reviewSummary struct {
	agreedApprovers    []string
	agreedReviewers    []string
	disagreedApprovers []string
	disagreedReviewers []string
}

func (r reviewSummary) NumberOfAssentor() int {
	return len(r.agreedApprovers) + len(r.agreedReviewers)
}

func (r reviewSummary) IsEmpty() bool {
	v := []int{
		len(r.agreedApprovers),
		len(r.agreedReviewers),
		len(r.disagreedApprovers),
		len(r.disagreedReviewers),
	}
	for _, item := range v {
		if item > 0 {
			return false
		}
	}
	return true
}

type reviewCommand struct {
	author  string
	command string
}

func genReviewSummary(cmds []reviewCommand) reviewSummary {
	agreedApprovers := sets.NewString()
	agreedReviewers := sets.NewString()
	disagreedApprovers := sets.NewString()
	disagreedReviewers := sets.NewString()
	for _, c := range cmds {
		switch c.command {
		case cmdLGTM:
			agreedReviewers.Insert(c.author)
		case cmdAPPROVE:
			agreedApprovers.Insert(c.author)
		case cmdReject:
			disagreedApprovers.Insert(c.author)
		case cmdLBTM:
			disagreedReviewers.Insert(c.author)
		}
	}

	return reviewSummary{
		agreedApprovers:    agreedApprovers.List(),
		agreedReviewers:    agreedReviewers.List(),
		disagreedApprovers: disagreedApprovers.List(),
		disagreedReviewers: disagreedReviewers.List(),
	}
}

type reviewResult struct {
	isRejected  bool
	isApproved  bool
	isLGTM      bool
	isLBTM      bool
	needLGTMNum int
}

func genReviewResult(r reviewSummary, allFilesApproved func([]string, int) bool, cfg reviewConfig) reviewResult {
	rr := reviewResult{}

	if len(r.disagreedApprovers) > 0 {
		rr.isRejected = true
		return rr
	}

	an := len(r.agreedApprovers)

	if allFilesApproved(r.agreedApprovers, cfg.NumberOfApprovers) {
		rr.isApproved = an >= cfg.TotalNumberOfApprovers
	}

	rn := an + len(r.agreedReviewers)
	f := func() {
		rr.isLGTM = rn >= cfg.TotalNumberOfReviewers
		if !rr.isLGTM {
			rr.needLGTMNum = cfg.TotalNumberOfReviewers - rn
		}
	}

	if rr.isApproved {
		// overrule the lbtm
		f()
		return rr
	}

	if rn < len(r.disagreedReviewers) {
		rr.isLBTM = true
	} else {
		f()
	}
	return rr
}

func multiError() *utils.MultiError {
	return utils.NewMultiErrors()
}

type iPRInfo interface {
	getOrgAndRepo() (string, string)
	getNumber() int32
	getTargetBranch() string
	hasLabel(string) bool
	getAuthor() string
	getHeadSHA() string
}

func newCommentInfo(comment, commenter string) *commentInfo {
	info := &commentInfo{
		comment:    comment,
		commenter:  normalizeLogin(commenter),
		authorCmds: sets.NewString(),
		reviewCmds: sets.NewString(),
	}
	for _, match := range commandRegex.FindAllStringSubmatch(comment, -1) {
		cmd := strings.ToUpper(match[1])

		if reviewCmds.Has(cmd) {
			info.reviewCmds.Insert(cmd)
		}

		if authorCmds.Has(cmd) {
			info.authorCmds.Insert(cmd)
		}
	}

	return info
}

type commentInfo struct {
	comment    string
	commenter  string
	authorCmds sets.String
	reviewCmds sets.String
}

func (c *commentInfo) hasReviewCmd() bool {
	return len(c.reviewCmds) > 0
}

func (c *commentInfo) hasCanReviewCmd() bool {
	return c.authorCmds.Has(cmdCanReview)
}

func (c *commentInfo) hasAssignCmd() bool {
	return false
}

func (c *commentInfo) validateReviewCmd(isValidCmd func(cmd, author string) bool) (validCmd string, invalidCmd string) {
	cmds := c.reviewCmds.UnsortedList()
	if len(cmds) == 0 {
		return
	}

	negatives := map[string]bool{}
	positives := map[string]bool{}

	for _, cmd := range cmds {
		if !isValidCmd(cmd, c.commenter) {
			if invalidCmd == "" {
				invalidCmd = cmd
			}
			continue
		}

		validCmd = cmd

		if negativeCmds.Has(cmd) {
			negatives[cmd] = true
		}
		if positiveCmds.Has(cmd) {
			positives[cmd] = true
		}
	}

	if len(negatives) == 0 && positives[cmdAPPROVE] {
		validCmd = cmdAPPROVE
	}
	return
}
