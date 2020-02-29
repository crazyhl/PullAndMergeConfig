package main

import (
	"github.com/fatih/color"
	"gopkg.in/yaml.v2"
	"io/ioutil"
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
	hiYellowColor.Println("开始加载配置文件")
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

	if len(parseConfig.BaseRule) <= 0 {
		hiRedColor.Println("基础规则源不存在，请填写配置文件后在获取")
		return
	}
	baseRules := parseConfig.BaseRule
	// 先获取第一个配置当做默认配置
	getRule := baseRules[0]
	// 获取参数
	inputArgs := os.Args
	if len(inputArgs) < 2 {
		cyanColor.Println("没有输入任何参数，将获取第一个规则当做拉取规则，规则名称： " + getRule.Name)
	}

	if len(inputArgs) > 1 {
		inputArgBaseRuleName := inputArgs[1]
		rule, err := getItems(baseRules, inputArgBaseRuleName)
		if err != nil {
			hiRedColor.Println(err)
			return
		}
		parseRule, ruleErr := rule.(model.BaseRule)
		if !ruleErr {
			hiRedColor.Println("基础规则源解析失败，请检查配置文件！")
			return
		}
		getRule = parseRule
	}
	cyanColor.Println(getRule)
	//rule, err := baseRules.HasItem("Hackl0us1");

}


func getItems(items my_interface.HasItemInterface, name string) (interface{}, error) {
	return items.HasItem(name)
}