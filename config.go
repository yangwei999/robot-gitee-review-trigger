package main

import (
	"errors"

	"github.com/opensourceways/community-robot-lib/config"
)

type configuration struct {
	ConfigItems []botConfig `json:"config_items,omitempty"`

	// CommandsEndpoint is the endpoint which enumerates the usage of commands.
	CommandsEndpoint string `json:"commands_endpoint" required:"true"`

	// Doc describes useful information about review process of PR.
	Doc string `json:"doc"`

	Maintainers map[string][]string `json:"maintainers"`
}

func (c *configuration) configFor(org, repo string) *botConfig {
	if c == nil {
		return nil
	}

	items := c.ConfigItems
	v := make([]config.IRepoFilter, len(items))
	for i := range items {
		v[i] = &items[i]
	}

	if i := config.Find(org, repo, v); i >= 0 {
		item := &items[i]
		item.doc = c.Doc
		item.commandsEndpoint = c.CommandsEndpoint
		if c.Maintainers != nil {
			item.maintainers = c.Maintainers[org+"/"+repo]
		}

		return item
	}

	return nil
}

func (c *configuration) Validate() error {
	if c == nil {
		return nil
	}

	if c.CommandsEndpoint == "" {
		return errors.New("missing commands_endpoint")
	}

	items := c.ConfigItems
	for i := range items {
		if err := items[i].validate(); err != nil {
			return err
		}
	}

	return nil
}

func (c *configuration) SetDefault() {
	if c == nil {
		return
	}

	Items := c.ConfigItems
	for i := range Items {
		Items[i].setDefault()
	}
}

type botConfig struct {
	config.RepoFilter

	Review reviewConfig `json:"review"`

	CLALabel string `json:"cla_label" required:"true"`

	// LabelsForBasicCIPassed is the label name for org/repos indicating
	// the basic CI test cases have passed
	LabelsForBasicCIPassed []string `json:"labels_for_basic_ci_passed,omitempty"`

	// NeedWelcome specifies whether to add welcome comment.
	NeedWelcome bool `json:"need_welcome,omitempty"`

	// CheckPRInfo  specifies whether to update tester number and add label.
	CheckPRInfo bool `json:"check_pr_info,omitempty"`

	doc              string   `json:"-"`
	maintainers      []string `json:"-"`
	commandsEndpoint string   `json:"-"`
}

func (c *botConfig) setDefault() {
	if c != nil {
		c.Review.setDefault()
	}
}

func (c *botConfig) validate() error {
	if c == nil {
		return nil
	}

	if c.CLALabel == "" {
		return errors.New("missing cla_label")
	}

	if err := c.Review.validate(); err != nil {
		return err
	}

	return c.RepoFilter.Validate()
}
