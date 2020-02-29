package main

import (
	"fmt"
	"github.com/fatih/color"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
	"parseAndCombineMyClashRules/model"
	"parseAndCombineMyClashRules/my_interface"
)

func main() {
	//	vmessStr := "123"
	//	vmessByteArr, err := base64.URLEncoding.DecodeString(vmessStr)
	//	fmt.Println(string(vmessByteArr))
	//	fmt.Println("---" + err.Error() + "------")
	// 定义一波颜色
	hiYellowColor := color.New(color.FgHiYellow)
	hiRedColor := color.New(color.FgHiRed)
	cyanColor := color.New(color.FgCyan)
	// 读入配置文件
	hiYellowColor.Println("开始加载配置文件...")
	configFile, err := ioutil.ReadFile("./config.yaml")
	if err != nil {
		hiRedColor.Printf("error: %v\n", err)
		return
	}
	parseConfig := model.Config{}
	err = yaml.Unmarshal(configFile, &parseConfig)
	if err != nil {
		hiRedColor.Printf("error: %v\n", err)
		return
	}

	if len(parseConfig.ConfigBaseRule) <= 0 {
		hiRedColor.Println("基础规则源不存在，请填写配置文件后在获取")
		return
	}
	//cyanColor.Println(parseConfig.FallbackFilter)
	//return
	// 获取基础配置列表
	baseRules := parseConfig.ConfigBaseRule
	// 先获取第一个配置当做默认配置
	getRule := baseRules[0]
	// 获取参数
	inputArgs := os.Args
	if len(inputArgs) < 2 {
		cyanColor.Println("没有输入任何参数，将获取第一个规则当做拉取规则，规则名称： " + getRule.Name)
	}
	// 如果有传入参数，就根据参数获取拉取规则的配置
	if len(inputArgs) > 1 {
		inputArgBaseRuleName := inputArgs[1]
		rule, err := getItems(baseRules, inputArgBaseRuleName)
		if err != nil {
			hiRedColor.Println(err)
			return
		}
		parseRule, ruleErr := rule.(model.ConfigBaseRule)
		if !ruleErr {
			hiRedColor.Println("基础规则源解析失败，请检查配置文件！")
			return
		}
		getRule = parseRule
		cyanColor.Println("采用输入的规则，规则名称： " + getRule.Name)
	}
	// 获取 最新的规则
	hiYellowColor.Println("开始拉取基础规则...")
	baseRuleBody, baseRuleErr := httpGet(getRule.Url)
	if baseRuleErr != nil {
		hiRedColor.Println(baseRuleErr)
		return
	}
	// 规则获取完成后多线程拉取用户订阅
	// 写出规则文件
	parseRule := model.Rule{}
	err = yaml.Unmarshal(baseRuleBody, &parseRule)
	if err != nil {
		hiRedColor.Printf("error: %v\n", err)
		return
	}
	d, _ := yaml.Marshal(&parseRule)

	fmt.Printf("--- m dump:\n%s\n\n", string(d))
}

func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// 通用的获取 items 方法
func getItems(items my_interface.HasItemInterface, name string) (interface{}, error) {
	return items.HasItem(name)
}
