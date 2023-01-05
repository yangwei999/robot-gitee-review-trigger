package main

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

func updatePRLabel(c ghclient, pr iPRInfo, keep ...string) error {
	all := sets.NewString(labelApproved, labelLGTM, labelRequestChange)
	_, err := updateAndReturnRemovedLabels(c, pr, all, keep...)

	return err
}

func removeAllLabels(c ghclient, pr iPRInfo, keep []string) ([]string, error) {
	all := sets.NewString(labelApproved, labelLGTM, labelApproved, labelRequestChange)

	return updateAndReturnRemovedLabels(c, pr, all, keep...)
}

func updateAndReturnRemovedLabels(c ghclient, pr iPRInfo, all sets.String, keep ...string) ([]string, error) {
	l := labelUpdating{
		c:  c,
		pr: pr,
	}

	mr := multiError()

	if err := l.addLabels(keep...); err != nil {
		mr.AddError(err)
	}

	toRemove := all.Delete(keep...).UnsortedList()

	removed, err := l.removeLabels(toRemove)
	if err != nil {
		mr.AddError(err)
	}

	return removed, mr.Err()
}

type labelUpdating struct {
	c  ghclient
	pr iPRInfo
}

func (l labelUpdating) addLabels(labels ...string) error {
	pr := l.pr

	toAdd := filterSlice(labels, pr.hasLabel)
	if len(toAdd) == 0 {
		return nil
	}

	org, repo := pr.getOrgAndRepo()

	return l.c.AddMultiPRLabel(org, repo, pr.getNumber(), labels)
}

func (l labelUpdating) removeLabels(labels []string) ([]string, error) {
	pr := l.pr

	toRemove := filterSlice(labels, func(l string) bool {
		return !pr.hasLabel(l)
	})
	if len(toRemove) == 0 {
		return nil, nil
	}

	org, repo := pr.getOrgAndRepo()

	return toRemove, l.c.RemovePRLabels(org, repo, pr.getNumber(), toRemove)
}

func filterSlice(s []string, filter func(string) bool) []string {
	v := []string{}
	for _, l := range s {
		if !filter(l) {
			v = append(v, l)
		}
	}
	return v
}
