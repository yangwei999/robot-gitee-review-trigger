package main

import (
	"fmt"

	"github.com/opensourceways/community-robot-lib/config"
)

type configuration struct {
	ConfigItems []botConfig `json:"config_items,omitempty"`

	Email EmailConfig `json:"email"`

	// CommandsEndpoint is the endpoint which enumerates the usage of commands.
	CommandsEndpoint string `json:"commands_endpoint" required:"true"`

	// Doc describes useful information about review process of PR.
	Doc string `json:"doc" required:"true"`
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
		items[i].doc = c.Doc
		items[i].commandsEndpoint = c.CommandsEndpoint
		items[i].email = c.Email

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

	ReviewerEmails map[string]string `json:"reviewer_emails"`

	// NeedWelcome specifies whether to add welcome comment.
	NeedWelcome bool `json:"need_welcome,omitempty"`

	doc              string      `json:"-"`
	commandsEndpoint string      `json:"-"`
	email            EmailConfig `json:"-"`
}

type EmailConfig struct {
	AuthCode string `json:"auth_code"`
	From     string `json:"from"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
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
