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

	cmdLGTM    = "LGTM"
	cmdLBTM    = "LBTM"
	cmdAPPROVE = "APPROVE"
	cmdReject  = "REJECT"
)

var (
	validCmds            = sets.NewString(cmdLGTM, cmdLBTM, cmdAPPROVE, cmdReject)
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

func parseReviewCommand(comment string) (r []string) {
	v := parseCommentCommands(comment)
	if len(v) == 0 {
		return
	}

	r = make([]string, 0, len(v))
	for _, cmd := range v {
		if validCmds.Has(cmd) {
			r = append(r, cmd)
		}
	}

	return
}

func parseCommentCommands(comment string) (r []string) {
	for _, match := range commandRegex.FindAllStringSubmatch(comment, -1) {
		r = append(r, strings.ToUpper(match[1]))
	}

	return
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

func getReviewCommand(
	comment, author string,
	isValidCmd func(cmd, author string) bool,
) (validCmd string, invalidCmd string) {

	cmds := parseReviewCommand(comment)
	if len(cmds) == 0 {
		return
	}

	return checkReviewCommand(cmds, func(cmd string) bool {
		return isValidCmd(cmd, author)
	})

}

func checkReviewCommand(
	cmds []string,
	isValidCmd func(cmd string) bool,
) (validCmd string, invalidCmd string) {

	negatives := map[string]bool{}
	positives := map[string]bool{}

	for _, cmd := range cmds {
		if !isValidCmd(cmd) {
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
	hasAnyLabel([]string) bool
	getAuthor() string
	getHeadSHA() string
	getUrl() string
	getTitle() string
}
