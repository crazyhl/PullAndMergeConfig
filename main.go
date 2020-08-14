package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"parseAndCombineMyClashRules/utils"
	"path/filepath"
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
	// 解析本地配置文件
	userConfigMap := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(configFile), &userConfigMap)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// 获取请求参数的配置项
	baseConfigName := r.URL.Query().Get("baseName")
	baseConfigRequestUrl := ""

	if baseConfigName != "" {
		// 如果获取成功就去找配置文件
		for _, baseConfig := range userConfigMap["base-config"].([]interface{}) {
			baseConfigMap := baseConfig.(map[interface{}]interface{})
			if baseConfigMap["name"] == baseConfigName {
				baseConfigRequestUrl = baseConfigMap["url"].(string)
			}
		}
	} else {
		log.Printf("请求url: %s，请求IP: %s\n，没有输入配置文件名称", r.URL, utils.GetRequestIp(r))
		return
	}

	if baseConfigRequestUrl == "" {
		log.Printf("请求url: %s，请求IP: %s\n，没有找到对应配置文件", r.URL, utils.GetRequestIp(r))
	}

	// 拉取基础配置
	baseRuleBody, baseRuleErr := utils.HttpGet(baseConfigRequestUrl)
	if baseRuleErr != nil {
		log.Println(baseRuleErr)
		log.Printf("请求url: %s，请求IP: %s\n，拉取基础规则失败，请检查网络！！！！", r.URL, utils.GetRequestIp(r))
		return
	}

	baseRuleMap := make(map[string]interface{})
	unmarshalReadyToWriteRuleErr := yaml.Unmarshal(baseRuleBody, &baseRuleMap)
	if unmarshalReadyToWriteRuleErr != nil {
		log.Printf("error: %v\n", unmarshalReadyToWriteRuleErr)
		log.Println("解析基础规则失败，请检查 url 是否正确 ！！！")
		return
	}

	fmt.Println(baseRuleMap)

	//
	//// 规则获取完成后多线程拉取用户订阅
	//pullProxySourceCunt := len(customConfig.PullProxySource)
	//wg := sync.WaitGroup{}
	//wg.Add(pullProxySourceCunt)
	//proxyArr := concurrent_map.New()
	//for i := 0; i < pullProxySourceCunt; i++ {
	//	go func(source model.PullProxySource) {
	//		defer wg.Done()
	//		proxyArr.Set(source.Name, nil)
	//		proxyBody, proxyErr := utils.HttpGet(source.Url)
	//		if proxyErr != nil {
	//			return
	//		}
	//
	//		decodeProxy, decodeProxyErr := base64.URLEncoding.DecodeString(string(proxyBody))
	//		if decodeProxyErr != nil {
	//			//_, _ = hiMagenta.Println(source.Name + "订阅 Proxy 信息不是 base64 文件，尝试用yaml解析")
	//			yamlProxyArr, yamlProxyErr := utils.ParseYamlProxy(proxyBody, customConfig.FilterProxyName, customConfig.FilterProxyServer)
	//			if yamlProxyErr != nil {
	//				//_, _ = hiMagenta.Println(source.Name + yamlProxyErr.Error())
	//				return
	//			}
	//			proxyArr.Set(source.Name, yamlProxyArr)
	//			//_, _ = hiMagenta.Println(source.Name + "获取节点信息成功， yaml 格式。")
	//			return
	//		}
	//		//_, _ = hiMagenta.Println(source.Name + "订阅 Proxy 信息是 base64 文件，尝试用 base64 解析")
	//		base64ProxyServerArr, base64ProxyServerErr := utils.ParseBase64Proxy(decodeProxy, customConfig.FilterProxyName, customConfig.FilterProxyServer)
	//		if base64ProxyServerErr != nil {
	//			//_, _ = hiMagenta.Println(source.Name + base64ProxyServerErr.Error())
	//			return
	//		}
	//		proxyArr.Set(source.Name, base64ProxyServerArr)
	//		//_, _ = hiMagenta.Println(source.Name + "获取节点信息成功， base64 格式。")
	//		return
	//	}(customConfig.PullProxySource[i])
	//}
	//wg.Wait()
	//// 写出规则文件
	//if configBaseRule.Name == "Hackl0us" {
	//	baseRule := base_rule.Hackl0us{Rule: readyToWriteRule}
	//	readyToWriteRule = my_interface.MergeBaseRule(baseRule, customConfig, proxyArr.Map)
	//} else if configBaseRule.Name == "ConnersHua" {
	//	baseRule := base_rule.ConnersHua{Rule: readyToWriteRule}
	//	readyToWriteRule = my_interface.MergeBaseRule(baseRule, customConfig, proxyArr.Map)
	//}
	//
	//marshalRule, _ := yaml.Marshal(&readyToWriteRule)
	//fmt.Fprintln(w, string(marshalRule))
}
