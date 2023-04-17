package main

import (
	"errors"

	"github.com/opensourceways/community-robot-lib/config"
	"github.com/opensourceways/community-robot-lib/robot-gitee-framework"
	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/opensourceways/repo-owners-cache/grpc/client"
	"github.com/sirupsen/logrus"
)

const botName = "review-trigger"

func newRobot(cli iClient, cacheCli *client.Client, botName string) *robot {
	return &robot{
		client:   ghclient{cli},
		botName:  botName,
		cacheCli: cacheCli,
	}
}

type iClient interface {
	AddPRLabel(owner, repo string, number int32, label string) error
	AddMultiPRLabel(org, repo string, number int32, label []string) error
	RemovePRLabel(owner, repo string, number int32, label string) error
	RemovePRLabels(org, repo string, number int32, labels []string) error
	GetPRCommit(org, repo, SHA string) (sdk.RepoCommit, error)
	ListPRComments(org, repo string, number int32) ([]sdk.PullRequestComments, error)
	GetPRLabels(org, repo string, number int32) ([]sdk.Label, error)
	CreatePRComment(owner, repo string, number int32, comment string) error
	DeletePRComment(org, repo string, ID int32) error
	UpdatePRComment(org, repo string, commentID int32, comment string) error
	GetPullRequestChanges(org, repo string, number int32) ([]sdk.PullRequestFiles, error)
	ListCollaborators(org, repo string) ([]sdk.ProjectMember, error)
	ListPROperationLogs(org, repo string, number int32) ([]sdk.OperateLog, error)
}

type robot struct {
	botName  string
	client   ghclient
	cacheCli *client.Client
}

func (bot *robot) NewConfig() config.Config {
	return &configuration{}
}

func (bot *robot) getConfig(cfg config.Config) (*configuration, error) {
	if c, ok := cfg.(*configuration); ok {
		return c, nil
	}
	return nil, errors.New("can't convert to configuration")
}

func (bot *robot) RegisterEventHandler(f framework.HandlerRegitster) {
	f.RegisterPullRequestHandler(bot.handlePREvent)
	f.RegisterNoteEventHandler(bot.handleNoteEvent)
}

func (bot *robot) handlePREvent(e *sdk.PullRequestEvent, c config.Config, log *logrus.Entry) error {
	cfg, err := bot.getConfig(c)
	if err != nil {
		return err
	}

	bc := cfg.configFor(e.GetOrgRepo())
	if bc == nil {
		return nil
	}

	return bot.processPREvent(e, bc, log)
}

func (bot *robot) handleNoteEvent(e *sdk.NoteEvent, c config.Config, log *logrus.Entry) error {
	cfg, err := bot.getConfig(c)
	if err != nil {
		return err
	}

	bc := cfg.configFor(e.GetOrgRepo())
	if bc == nil {
		return nil
	}

	return bot.processNoteEvent(e, bc, log)
}
