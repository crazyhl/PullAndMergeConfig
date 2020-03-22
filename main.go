package main

import (
	"encoding/base64"
	"github.com/fatih/color"
	"github.com/upyun/go-sdk/upyun"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"parseAndCombineMyClashRules/base_rule"
	"parseAndCombineMyClashRules/model"
	"parseAndCombineMyClashRules/my_interface"
	"parseAndCombineMyClashRules/utils"
	"path/filepath"
	"sync"
)

func main() {
	// 定义输出颜色
	hiYellowColor := color.New(color.FgHiYellow)
	hiRedColor := color.New(color.FgHiRed)
	cyanColor := color.New(color.FgCyan)
	hiMagenta := color.New(color.FgHiMagenta)
	// 可执行程序所在目录
	root := filepath.Dir(os.Args[0])
	absPath, absPathErr := filepath.Abs(root)
	if absPathErr != nil {
		hiRedColor.Println("获取运行路径失败")
		return
	}
	// 读入配置文件
	_, _ = hiYellowColor.Println("开始加载配置文件...")
	configFile, readConfigFileErr := ioutil.ReadFile(absPath + "/config.yaml")
	if readConfigFileErr != nil {
		_, _ = hiRedColor.Printf("error: %v\n", readConfigFileErr)
		_, _ = hiRedColor.Println("读入配置文件失败了，检查配置文件是否存在")
		return
	}
	// 解析自定义的配置文件
	customConfig := model.Config{}
	unmarshalConfigErr := yaml.Unmarshal(configFile, &customConfig)
	if unmarshalConfigErr != nil {
		_, _ = hiRedColor.Printf("error: %v\n", unmarshalConfigErr)
		_, _ = hiRedColor.Println("解析配置文件失败了，看看格式是不是不正确了")
		return
	}
	// 判断是否配置了要获取的根基配置文件是否存在
	if len(customConfig.ConfigBaseRule) <= 0 {
		_, _ = hiRedColor.Println("基础规则源不存在，请填写配置文件后在获取")
		return
	}
	// 获取基础配置列表
	baseRules := customConfig.ConfigBaseRule
	// 先获取第一个配置当做默认配置
	configBaseRule := baseRules[0]
	// 获取参数
	inputArgs := os.Args
	if len(inputArgs) < 2 {
		_, _ = cyanColor.Println("没有输入任何参数，将获取第一个规则当做拉取规则，规则名称： " + configBaseRule.Name)
	}
	// 如果有传入参数，就根据参数获取拉取规则的配置
	if len(inputArgs) > 1 {
		inputArgBaseRuleName := inputArgs[1]
		rule, err := my_interface.GetItems(baseRules, inputArgBaseRuleName)
		if err != nil {
			_, _ = hiRedColor.Println(err)
			return
		}
		// 通过断言进行类型转换，把空接口转换为我们的model类型
		parseConfigBaseRule, assertErr := rule.(model.ConfigBaseRule)
		if !assertErr {
			_, _ = hiRedColor.Println(assertErr)
			_, _ = hiRedColor.Println("基础规则源解析失败，请检查配置文件！")
			return
		}
		configBaseRule = parseConfigBaseRule
		_, _ = cyanColor.Println("采用输入的规则，规则名称： " + configBaseRule.Name)
	}
	// 获取 最新的规则
	_, _ = hiYellowColor.Println("开始拉取基础规则")
	baseRuleBody, baseRuleErr := utils.HttpGet(configBaseRule.Url)
	if baseRuleErr != nil {
		_, _ = hiRedColor.Println(baseRuleErr)
		_, _ = hiRedColor.Println("拉取基础规则失败，请检查网络！！！！")
		return
	}
	readyToWriteRule := model.Rule{}
	unmarshalReadyToWriteRuleErr := yaml.Unmarshal(baseRuleBody, &readyToWriteRule)
	if unmarshalReadyToWriteRuleErr != nil {
		_, _ = hiRedColor.Printf("error: %v\n", unmarshalReadyToWriteRuleErr)
		_, _ = hiRedColor.Println("解析基础规则失败，请检查 url 是否正确 ！！！")
		return
	}
	// 规则获取完成后多线程拉取用户订阅
	_, _ = hiYellowColor.Println("开始拉取订阅 Proxy 信息")
	pullProxySourceCunt := len(customConfig.PullProxySource)
	wg := sync.WaitGroup{}
	wg.Add(pullProxySourceCunt)
	proxyArr := make(map[string][]model.Proxy)
	for i := 0; i < pullProxySourceCunt; i++ {
		go func(source model.PullProxySource) {
			defer wg.Done()
			proxyArr[source.Name] = nil
			proxyBody, proxyErr := utils.HttpGet(source.Url)
			if baseRuleErr != nil {
				_, _ = hiRedColor.Println(proxyErr)
				_, _ = hiRedColor.Println(source.Name + "获取订阅 Proxy 信息，请检查网络！！！")
				return
			}

			decodeProxy, decodeProxyErr := base64.URLEncoding.DecodeString(string(proxyBody))
			if decodeProxyErr != nil {
				_, _ = hiMagenta.Println(source.Name + "订阅 Proxy 信息不是 base64 文件，尝试用yaml解析")
				yamlProxyArr, yamlProxyErr := utils.ParseYamlProxy(proxyBody, customConfig.FilterProxyName, customConfig.FilterProxyServer)
				if yamlProxyErr != nil {
					_, _ = hiMagenta.Println(source.Name + yamlProxyErr.Error())
					return
				}
				proxyArr[source.Name] = yamlProxyArr
				_, _ = hiMagenta.Println(source.Name + "获取节点信息成功， yaml 格式。")
				return
			}
			_, _ = hiMagenta.Println(source.Name + "订阅 Proxy 信息是 base64 文件，尝试用 base64 解析")
			base64ProxyServerArr, base64ProxyServerErr := utils.ParseBase64Proxy(decodeProxy, customConfig.FilterProxyName, customConfig.FilterProxyServer)
			if base64ProxyServerErr != nil {
				_, _ = hiMagenta.Println(source.Name + base64ProxyServerErr.Error())
				return
			}
			proxyArr[source.Name] = base64ProxyServerArr
			_, _ = hiMagenta.Println(source.Name + "获取节点信息成功， base64 格式。")
			return
		}(customConfig.PullProxySource[i])
	}
	wg.Wait()
	_, _ = hiYellowColor.Println("开始合并配置文件")

	// 写出规则文件
	if configBaseRule.Name == "Hackl0us" {
		baseRule := base_rule.Hackl0us{Rule: readyToWriteRule}
		readyToWriteRule = my_interface.MergeBaseRule(baseRule, customConfig, proxyArr)
	} else if configBaseRule.Name == "ConnersHua" {
		baseRule := base_rule.ConnersHua{Rule: readyToWriteRule}
		readyToWriteRule = my_interface.MergeBaseRule(baseRule, customConfig, proxyArr)
	}

	marshalRule, _ := yaml.Marshal(&readyToWriteRule)
	writeConfigPath := absPath + "/" + configBaseRule.Name + ".yaml"
	f, err := os.Create(writeConfigPath)
	if err != nil {
		_, _ = hiRedColor.Println(err)
		_, _ = hiRedColor.Println("创建写出文件失败,请检查是否存在同名文件！！！")
	} else {
		defer f.Close()

		_, err = f.Write(marshalRule)
		if err != nil {
			_, _ = hiRedColor.Println(err)
			_, _ = hiRedColor.Println("写入文件失败，请检查报错信息！！！")
		} else {
			_, _ = hiYellowColor.Println("配置文件写出成功，快复制到 clash 的配置文件夹使用吧!!!")
			// 写出成功后，判断是否需要上传
			uploadConfig := customConfig.UploadConfig
			if uploadConfig == "upyun" {
				// 如果输入了配置就读入配置
				_, _ = hiYellowColor.Println("采用又拍云配置上传。。。")
				appointConfig := customConfig.UpYunConfig
				if appointConfig.Bucket == "" || appointConfig.Operator == "" || appointConfig.Password == "" {
					_, _ = hiRedColor.Println("又拍云参数错误，bucket、operator 和 password 为必填")
					return
				}

				up := upyun.NewUpYun(&upyun.UpYunConfig{
					Bucket:   appointConfig.Bucket,
					Operator: appointConfig.Operator,
					Password: appointConfig.Password,
				})

				upYunUploadConfigError := up.Put(&upyun.PutObjectConfig{
					Path:      appointConfig.PathPrefix + configBaseRule.Name + ".yaml",
					LocalPath: writeConfigPath,
				})
				if upYunUploadConfigError != nil {
					_, _ = hiRedColor.Println("上传错误" + upYunUploadConfigError.Error())
					return
				} else {
					_, _ = hiYellowColor.Println("又拍云配置上传成功")
				}
			}
		}
	}
}
