package main

import (
	"encoding/base64"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"parseAndCombineMyClashRules/base_rule"
	"parseAndCombineMyClashRules/concurrent_map"
	"parseAndCombineMyClashRules/model"
	"parseAndCombineMyClashRules/my_interface"
	"parseAndCombineMyClashRules/utils"
	"path/filepath"
	"sync"
)

var absPath string
var absPathErr error

func init() {
	// 设置日志输出前缀
	log.SetPrefix("TRACE: ")
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Llongfile)
	// 可执行程序所在目录
	root := filepath.Dir(os.Args[0])
	absPath, absPathErr = filepath.Abs(root)
	// 创建日志目录
	os.MkdirAll(absPath+"/log", os.ModePerm)
	// 设置日志输出文件
	logFile, logFileErr := os.OpenFile(absPath+"/log/errors.txt",
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if logFileErr != nil {
		log.Printf("error: %v\n", logFileErr)
		panic("创建日志文件失败")
	}
	log.SetOutput(logFile)
}

func main() {
	if absPathErr != nil {
		fmt.Println("获取运行路径失败")
		return
	}

	// 获取参数
	inputArgs := os.Args
	if len(inputArgs) < 2 {
		fmt.Println("请输入参数")
		return
	}

	port := inputArgs[1]
	fmt.Println("服务启动，端口：" + port)
	http.HandleFunc("/parse", parseRule)
	http.ListenAndServe("0.0.0.0:"+port, nil)
}

func parseRule(w http.ResponseWriter, r *http.Request) {
	// 获取配置参数
	configFileNameArr, getConfigFileNameOk := r.URL.Query()["name"]
	if getConfigFileNameOk == false {
		return
	}
	configFileName := configFileNameArr[0]
	configFile, readConfigFileErr := ioutil.ReadFile(absPath + "/config/" + configFileName + ".yaml")
	if readConfigFileErr != nil {
		log.Printf("error: %v\n", readConfigFileErr)
		log.Println("读入配置文件失败了，检查配置文件是否存在")
		return
	}

	// 解析自定义的配置文件
	customConfig := model.Config{}
	unmarshalConfigErr := yaml.Unmarshal(configFile, &customConfig)
	if unmarshalConfigErr != nil {
		log.Printf("error: %v\n", unmarshalConfigErr)
		log.Println("解析配置文件失败了，看看格式是不是不正确了")
		return
	}

	// 判断是否配置了要获取的根基配置文件是否存在
	if len(customConfig.ConfigBaseRule) <= 0 {
		log.Println("基础规则源不存在，请填写配置文件后在获取")
		return
	}
	// 获取基础配置列表
	baseRules := customConfig.ConfigBaseRule
	// 先获取第一个配置当做默认配置
	configBaseRule := baseRules[0]
	// 获取参数
	baseConfigNameArr, getBaseConfigNameArrOk := r.URL.Query()["baseName"]
	if getBaseConfigNameArrOk {
		configBaseRuleName := baseConfigNameArr[0]
		rule, err := my_interface.GetItems(baseRules, configBaseRuleName)
		if err != nil {
			log.Println(err)
			return
		}
		// 通过断言进行类型转换，把空接口转换为我们的model类型
		parseConfigBaseRule, assertErr := rule.(model.ConfigBaseRule)
		if !assertErr {
			log.Println(assertErr)
			log.Println("基础规则源解析失败，请检查配置文件！")
			return
		}
		configBaseRule = parseConfigBaseRule
	}

	// 获取 最新的规则
	baseRuleBody, baseRuleErr := utils.HttpGet(configBaseRule.Url)
	if baseRuleErr != nil {
		log.Println(baseRuleErr)
		log.Println("拉取基础规则失败，请检查网络！！！！")
		return
	}

	readyToWriteRule := model.Rule{}
	unmarshalReadyToWriteRuleErr := yaml.Unmarshal(baseRuleBody, &readyToWriteRule)
	if unmarshalReadyToWriteRuleErr != nil {
		log.Printf("error: %v\n", unmarshalReadyToWriteRuleErr)
		log.Println("解析基础规则失败，请检查 url 是否正确 ！！！")
		return
	}

	// 规则获取完成后多线程拉取用户订阅
	pullProxySourceCunt := len(customConfig.PullProxySource)
	wg := sync.WaitGroup{}
	wg.Add(pullProxySourceCunt)
	proxyArr := concurrent_map.New()
	for i := 0; i < pullProxySourceCunt; i++ {
		go func(source model.PullProxySource) {
			defer wg.Done()
			proxyArr.Set(source.Name, nil)
			proxyBody, proxyErr := utils.HttpGet(source.Url)
			if proxyErr != nil {
				return
			}

			decodeProxy, decodeProxyErr := base64.URLEncoding.DecodeString(string(proxyBody))
			if decodeProxyErr != nil {
				//_, _ = hiMagenta.Println(source.Name + "订阅 Proxy 信息不是 base64 文件，尝试用yaml解析")
				yamlProxyArr, yamlProxyErr := utils.ParseYamlProxy(proxyBody, customConfig.FilterProxyName, customConfig.FilterProxyServer)
				if yamlProxyErr != nil {
					//_, _ = hiMagenta.Println(source.Name + yamlProxyErr.Error())
					return
				}
				proxyArr.Set(source.Name, yamlProxyArr)
				//_, _ = hiMagenta.Println(source.Name + "获取节点信息成功， yaml 格式。")
				return
			}
			//_, _ = hiMagenta.Println(source.Name + "订阅 Proxy 信息是 base64 文件，尝试用 base64 解析")
			base64ProxyServerArr, base64ProxyServerErr := utils.ParseBase64Proxy(decodeProxy, customConfig.FilterProxyName, customConfig.FilterProxyServer)
			if base64ProxyServerErr != nil {
				//_, _ = hiMagenta.Println(source.Name + base64ProxyServerErr.Error())
				return
			}
			proxyArr.Set(source.Name, base64ProxyServerArr)
			//_, _ = hiMagenta.Println(source.Name + "获取节点信息成功， base64 格式。")
			return
		}(customConfig.PullProxySource[i])
	}
	wg.Wait()
	// 写出规则文件
	if configBaseRule.Name == "Hackl0us" {
		baseRule := base_rule.Hackl0us{Rule: readyToWriteRule}
		readyToWriteRule = my_interface.MergeBaseRule(baseRule, customConfig, proxyArr.Map)
	} else if configBaseRule.Name == "ConnersHua" {
		baseRule := base_rule.ConnersHua{Rule: readyToWriteRule}
		readyToWriteRule = my_interface.MergeBaseRule(baseRule, customConfig, proxyArr.Map)
	}

	marshalRule, _ := yaml.Marshal(&readyToWriteRule)
	fmt.Fprintln(w, string(marshalRule))
}
