package main

import (
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/opensourceways/repo-owners-cache/repoowners"
)

func newPullRequest(pr iPRInfo, files, assignees []string, owner repoowners.RepoOwner) pullRequest {
	fileApproverMap := map[string]sets.String{}
	fileReviewerMap := map[string]sets.String{}
	for _, path := range files {
		fileApproverMap[path] = owner.Approvers(path)
		fileReviewerMap[path] = owner.Reviewers(path)
	}

	m := map[string]sets.String{}
	n := map[string]sets.String{}
	for dir, v := range fileApproverMap {
		for item := range v {
			if _, ok := m[item]; !ok {
				m[item] = sets.NewString(dir)
			} else {
				m[item].Insert(dir)
			}
		}
	}

	for dir, v := range fileReviewerMap {
		for item := range v {
			if _, ok := n[item]; !ok {
				n[item] = sets.NewString(dir)
			} else {
				n[item].Insert(dir)
			}
		}
	}

	return pullRequest{
		fileApproverMap: fileApproverMap,
		approverFileMap: m,
		files:           files,
		assignees:       assignees,
		info:            pr,
		reviewerFileMap: n,
		fileReviewerMap: fileReviewerMap,
	}
}

type pullRequest struct {
	info            iPRInfo
	files           []string
	assignees       []string
	fileApproverMap map[string]sets.String
	approverFileMap map[string]sets.String
	reviewerFileMap map[string]sets.String
	fileReviewerMap map[string]sets.String
}

func (p pullRequest) isApprover(author string) bool {
	_, b := p.approverFileMap[author]
	return b
}

func (p pullRequest) filesApprovedBy(author string) sets.String {
	if v, b := p.approverFileMap[author]; b {
		return v
	}

	return sets.String{}
}

func (p pullRequest) approversOfFile(file string) sets.String {
	if v, b := p.fileApproverMap[file]; b {
		return v
	}

	return sets.String{}
}

func (p pullRequest) areAllFilesApproved(agreedApprovers []string, num int) bool {
	if num == 1 {
		return p.areAllFilesReviewed(agreedApprovers)
	}

	records := p.stats(agreedApprovers)

	if len(records) < p.numberOfFiles() {
		return false
	}

	for _, n := range records {
		if n < num {
			return false
		}
	}
	return true
}

func (p pullRequest) stats(agreedApprovers []string) map[string]int {
	r := map[string]int{}
	for _, a := range agreedApprovers {
		for k := range p.filesApprovedBy(a) {
			r[k] += 1
		}
	}
	return r
}

func (p pullRequest) areAllFilesReviewed(approvers []string) bool {
	m := map[string]bool{}
	for _, a := range approvers {
		for i := range p.filesApprovedBy(a) {
			if !m[i] {
				m[i] = true
			}
		}
	}
	return len(m) == p.numberOfFiles()
}

func (p pullRequest) numberOfFiles() int {
	return len(p.files)
}

func (p pullRequest) prAuthor() string {
	return normalizeLogin(p.info.getAuthor())
}

func (p pullRequest) isReviewwer(author string) bool {
	_, b := p.fileReviewerMap[author]
	return b
}

func (p pullRequest) areAllFilesCommented(agreedReviewers []string, num int) bool {
	if num == 1 {
		return p.areAllFilesReviewedBy(agreedReviewers)
	}

	records := p.stat(agreedReviewers)

	if len(records) < p.numberOfFiles() {
		return false
	}

	for _, n := range records {
		if n < num {
			return false
		}
	}
	return true
}

func (p pullRequest) stat(agreedReviewers []string) map[string]int {
	r := map[string]int{}
	for _, a := range agreedReviewers {
		for k := range p.filesReviewedBy(a) {
			r[k] += 1
		}
	}
	return r
}

func (p pullRequest) filesReviewedBy(author string) sets.String {
	if v, b := p.reviewerFileMap[author]; b {
		return v
	}

	return sets.String{}
}

func (p pullRequest) areAllFilesReviewedBy(reviewers []string) bool {
	m := map[string]bool{}
	for _, a := range reviewers {
		for i := range p.filesReviewedBy(a) {
			if !m[i] {
				m[i] = true
			}
		}
	}
	return len(m) == p.numberOfFiles()
}
