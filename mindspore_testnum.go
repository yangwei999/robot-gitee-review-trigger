package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"regexp"
	"strings"

	sdk "github.com/opensourceways/go-gitee/gitee"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	bug               = "bug"
	task              = "task"
	feature           = "feature"
	refactor          = "refactor"
	infoTittle        = "**What does this PR do / why do we need it**"
	typeTittle        = "**What type of PR is this?**"
	issueTittle       = "**Which issue(s) this PR fixes**"
	labelGoodRefactor = "good-refactor-case"
)

var (
	commonLabelRegex = regexp.MustCompile(`(?m)^/(kind)\s*(.*?)\s*$`)
)

// 如果符合条件，设置测试人数为：0
func (bot *robot) setTestNumber(text, org, repo, author string, number int32) error {
	//check pr body
	err := bot.checkPRBody(text, author)
	if err != nil {
		return bot.client.CreatePRComment(org, repo, number, err.Error())
	}

	//检查是否存在refactor类型，更新测试人数和打标签
	validTypes := sets.NewString(refactor, bug, feature, task)
	matches := commonLabelRegex.FindAllStringSubmatch(text, -1)
	for _, math := range matches {
		if validTypes.HasAny(math[2]) {
			if math[2] == refactor {
				err := bot.client.AddPRLabel(org, repo, number, labelGoodRefactor)
				if err != nil {
					logrus.Errorf("add label failed: %v", err)
				}
			}
			if math[2] == feature {
				err := bot.updateTestNumber(org, repo, number, 1)
				if err != nil {
					logrus.Errorf("update tester number failed: %v", err)
				}
			} else {
				err := bot.updateTestNumber(org, repo, number, 0)
				if err != nil {
					logrus.Errorf("update tester number failed: %v", err)
				}
			}
		}
	}

	return nil
}

// 设置测试人数
func (bot *robot) updateTestNumber(org, repo string, number int32, v int32) error {
	p := sdk.PullRequestUpdateParam{
		TestersNumber: &v,
	}

	_, err := bot.client.UpdatePullRequest(org, repo, number, p)
	if err != nil {
		return fmt.Errorf("error update : %v", err)
	}
	return nil
}

// 检查PR描述内容是否符合规范
func (bot *robot) checkPRBody(text, author string) error {
	tittle1 := strings.Index(text, infoTittle)
	tittle2 := strings.Index(text, issueTittle)
	if tittle1 == -1 {
		return fmt.Errorf("PR missing description information."+
			"\n\nPlease add description of this pull request in pull request header."+
			"\n1.The kind of the pull request must be filled.Availabel values **/kind feature**,**/kind bug**,**/kind refactor**,or **/kind task**."+
			"\n2.The description must be filled also under **What does this PR do / why do we need it** section."+
			"\n\n@%s,You cannot comment **/can-review** before you write an information."+
			"\nIf you still have any doubts, please consult @liuchongming74", author)
	}
	var prContent string
	if tittle2 == -1 {
		prContent = text[tittle1+len(infoTittle):]
	} else {
		prContent = text[tittle1+len(infoTittle) : tittle2]
	}
	prContent2 := strings.ReplaceAll(strings.ReplaceAll(prContent, "\r\n", ""), ":", "")
	if prContent2 == "" {
		return fmt.Errorf("The introduction to PR is empty."+
			"\n\nPlease add description of this pull request in pull request header."+
			"\n1.The kind of the pull request must be filled.Availabel values **/kind feature**,**/kind bug**,**/kind refactor**,or **/kind task**."+
			"\n2.The description must be filled also under **What does this PR do / why do we need it** section."+
			"\n\n@%s,You cannot comment **/can-review** before you write an introduction."+
			"\nIf you still have any doubts, please consult @liuchongming74", author)
	}

	// 检查标题下方的内容是否为 /kind bug, /kind task, /kind feature 或 /kind refactor
	prTittle1 := strings.Index(text, typeTittle)
	prTittle2 := strings.Index(text, infoTittle)
	if prTittle1 == -1 {
		return fmt.Errorf("PR does not have the content of \"What type of PR is this?\"."+
			"\n\nPlease add description of this pull request in pull request header."+
			"\n1.The kind of the pull request must be filled.Availabel values **/kind feature**,**/kind bug**,**/kind refactor**,or **/kind task**."+
			"\n2.The description must be filled also under **What does this PR do / why do we need it** section."+
			"\n\n@%s,You cannot comment **/can-review** before you write an introduction."+
			"\nIf you still have any doubts, please consult @liuchongming74", author)
	}
	prtype := text[prTittle1+len(typeTittle) : prTittle2]
	prTypes := strings.Replace(prtype, "\r\n", "", -1)
	// 使用正则表达式去除 <!-- ... --> 及其内容
	re := regexp.MustCompile(`<!--.*?-->`)
	prTypes = re.ReplaceAllString(prTypes, "")
	if prTypes == "" {
		return fmt.Errorf("PR type is missing."+
			"\n\nPlease add description of this pull request in pull request header."+
			"\n1.The kind of the pull request must be filled.Availabel values **/kind feature**,**/kind bug**,**/kind refactor**,or **/kind task**."+
			"\n2.The description must be filled also under **What does this PR do / why do we need it** section."+
			"\n\n@%s,You cannot comment **/can-review** before you write the type."+
			"\nIf you still have any doubts, please consult @liuchongming74", author)
	}
	newType := strings.Split(prTypes, "/kind")
	var result []string
	for _, s := range newType {
		trimmedString := strings.TrimSpace(s)
		if trimmedString != "" {
			result = append(result, trimmedString)
		}
	}
	validTypes := map[string]bool{
		bug:      true,
		task:     true,
		feature:  true,
		refactor: true,
	}

	prTypeCount := 0
	for _, Type := range result {
		if _, ok := validTypes[Type]; !ok {
			return fmt.Errorf("This type : **%s** is not a type specified by the template."+
				"\n\nPlease add description of this pull request in pull request header."+
				"\n1.The kind of the pull request must be filled.Availabel values **/kind feature**,**/kind bug**,**/kind refactor**,or **/kind task**."+
				"\n2.The description must be filled also under **What does this PR do / why do we need it** section."+
				"\n\n@%s,You cannot comment **/can-review** before you write the type."+
				"\nIf you still have any doubts, please consult @liuchongming74", Type, author) // PR 类型不在规定范围内，报错或进行相应处理
		}
		// 检查 "/kind bug"、"/kind task" 和 "/kind feature" 是否同时存在
		if Type == bug || Type == task || Type == feature {
			prTypeCount++
		}
	}
	if prTypeCount > 1 {
		return fmt.Errorf("Invalid PR information: Multiple type cannot coexist."+
			"\n\nPlease add description of this pull request in pull request header."+
			"\n1.The kind of the pull request must be filled.Availabel values **/kind feature**,**/kind bug**,**/kind refactor**,or **/kind task**."+
			"\n2.The description must be filled also under **What does this PR do / why do we need it** section."+
			"\n\n@%s,You cannot comment **/can-review** before you write the type."+
			"\nIf you still have any doubts, please consult @liuchongming74", author) // 同时存在多个不能共存类型标签，返回错误
	}

	return nil
}
