package main

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

func updatePRLabel(c ghclient, pr iPRInfo, keep ...string) error {
	_, err := updateAndReturnRemovedLabels(c, pr, keep...)
	return err
}

func updateAndReturnRemovedLabels(c ghclient, pr iPRInfo, keep ...string) ([]string, error) {
	l := labelUpdating{
		c:  c,
		pr: pr,
	}

	mr := multiError()

	if err := l.addLabels(keep...); err != nil {
		mr.AddError(err)
	}

	all := sets.NewString(labelApproved, labelLGTM, labelRequestChange)

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

	return l.addLabelsSeparately(pr, toAdd)
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

func (l labelUpdating) addLabelsSeparately(pr iPRInfo, labels []string) error {
	org, repo := pr.getOrgAndRepo()

	mr := multiError()
	for _, label := range labels {
		err := l.c.AddPRLabel(org, repo, pr.getNumber(), label)
		mr.AddError(err)
	}

	return mr.Err()
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
