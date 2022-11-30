package main

import (
	"context"
	"os"
	"strings"

	"github.com/antihax/optional"
	"github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
)

type Email struct {
	reviewerEmails map[string]string
	eConfig        EmailConfig
}

func NewEmailService(cfg *botConfig) *Email {
	return &Email{
		reviewerEmails: cfg.ReviewerEmails,
		eConfig:        cfg.email,
	}
}

func (e *Email) SendEmailToReviewers(users []string, content string) error {
	subject := "PR review invitation from Gitee"
	return e.SendCommon(users, subject, content)
}

func (e *Email) SendCommon(users []string, subject, content string) error {
	addresses := e.GetEmailAddress(users)
	if len(addresses) > 0 {
		return e.SendEmail(subject, content, addresses)
	}
	return nil
}

func (e *Email) GetEmailAddress(users []string) []string {
	f := func(user string) string {
		var addr string
		if a, ok := e.reviewerEmails[strings.ToLower(user)]; ok {
			addr = a
		} else {
			addr = e.GetEmailAddressFromGitee(user)
		}
		return addr
	}

	var address []string
	for _, user := range users {
		if addr := f(user); addr != "" {
			address = append(address, addr)
		}
	}
	return address
}

func (e *Email) GetEmailAddressFromGitee(user string) string {
	cfg := gitee.NewConfiguration()
	apiClient := gitee.NewAPIClient(cfg)

	token, err := os.ReadFile("./oauth")
	if err != nil {
		logrus.Errorf("read oauth file fail:%s", err)
		return ""
	}
	option := gitee.GetV5UsersUsernameOpts{
		AccessToken: optional.NewString(string(token)),
	}
	userDetail, _, err := apiClient.UsersApi.GetV5UsersUsername(context.Background(), user, &option)
	if err != nil {
		logrus.Errorf("get user email from gitee error:%s", err)
		return ""
	}
	return userDetail.Email
}

func (e *Email) SendEmail(subject, content string, addresses []string) error {
	d := gomail.NewDialer(e.eConfig.Host, e.eConfig.Port, e.eConfig.From, e.eConfig.AuthCode)

	message := gomail.NewMessage()
	message.SetHeader("From", e.eConfig.From)
	message.SetHeader("To", addresses...)
	message.SetHeader("Subject", subject)
	message.SetBody("text/plain", content)

	if err := d.DialAndSend(message); err != nil {
		return err
	}
	return nil
}
