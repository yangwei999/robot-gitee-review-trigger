package main

import (
	"fmt"

	"github.com/opensourceways/community-robot-lib/config"
	"k8s.io/apimachinery/pkg/util/sets"
)

type configuration struct {
	ConfigItems []botConfig `json:"config_items,omitempty"`

	RecommendReviewers RecommendReviewers `json:"recommend" required:"true"`

	// CommandsEndpoint is the endpoint which enumerates the usage of commands.
	CommandsEndpoint string `json:"commands_endpoint" required:"true"`

	// Doc describes useful information about review process of PR.
	Doc string `json:"doc" required:"true"`

	Maintainers map[string][]string `json:"maintainers" required:"true"`
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

	recommendCommunity := sets.NewString(c.RecommendReviewers.SupportCommunity...)
	if i := config.Find(org, repo, v); i >= 0 {
		items[i].doc = c.Doc
		items[i].maintainers = c.Maintainers[org+"/"+repo]
		items[i].commandsEndpoint = c.CommandsEndpoint

		if _, ok := recommendCommunity[org]; ok {
			items[i].recommendUrl = c.RecommendReviewers.Url
		}

		return &items[i]
	}

	return nil
}

func (c *configuration) Validate() error {
	if c == nil {
		return nil
	}

	if c.CommandsEndpoint == "" {
		return fmt.Errorf("missing commands_endpoint")
	}

	if c.Doc == "" {
		return fmt.Errorf("missing doc")
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

	CI ciConfig `json:"ci"`

	Review reviewConfig `json:"review"`

	CLALabel string `json:"cla_label" required:"true"`

	// NeedWelcome specifies whether to add welcome comment.
	NeedWelcome bool `json:"need_welcome,omitempty"`

	doc              string   `json:"-"`
	maintainers      []string `json:"-"`
	commandsEndpoint string   `json:"-"`
	recommendUrl     string   `json:"-"`
}

type RecommendReviewers struct {
	Url              string   `json:"url"`
	SupportCommunity []string `json:"support_community"`
}

func (c *botConfig) setDefault() {
	if c != nil {
		c.CI.setDefault()
		c.Review.setDefault()
	}
}

func (c *botConfig) validate() error {
	if c == nil {
		return nil
	}

	if err := c.CI.validate(); err != nil {
		return err
	}

	if err := c.Review.validate(); err != nil {
		return err
	}

	return c.RepoFilter.Validate()
}
