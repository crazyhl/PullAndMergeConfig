package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"parseAndCombineMyClashRules/concurrent_map"
	"parseAndCombineMyClashRules/model"
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

	// 获取配置文件输入的订阅地址
	pullProxySource := userConfigMap["pull-proxy-source"]
	if pullProxySource == nil {
		return
	}
	pullProxySourceMap := pullProxySource.([]interface{})
	pullProxySourceMapLength := len(pullProxySourceMap)
	if pullProxySourceMapLength <= 0 {
		log.Printf("请求url: %s，请求IP: %s\n，没有获取到订阅地址", r.URL, utils.GetRequestIp(r))
	}
	// 多线程获取订阅
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(pullProxySourceMapLength)
	proxyMap := concurrent_map.New()
	proxyGroupMap := concurrent_map.New()
	for _, proxySource := range pullProxySourceMap {
		go func(proxySource interface{}) {
			proxySourceMap := proxySource.(map[interface{}]interface{})
			// 获取订阅
			httpResponse, httpRequestErr := http.Get(proxySourceMap["url"].(string))
			if httpRequestErr != nil {
				log.Printf(
					"请求url: %s，请求IP: %s, 订阅地址: %s, 报错内容：%v\n，没有获取到订阅地址",
					r.URL,
					utils.GetRequestIp(r),
					proxySourceMap["url"].(string),
					httpRequestErr.Error(),
				)
			}
			fmt.Println(httpResponse)
			// 插入到自定义组
			proxyGroupMap.Set(proxySourceMap["name"].(string), nil)
			fmt.Println(proxySourceMap["name"])
			waitGroup.Done()
		}(proxySource)
	}
	waitGroup.Wait()
	fmt.Println(proxyGroupMap.Map)
	fmt.Println(proxyMap.Map)
	// 构造一些前置的参数
	outputConfig := model.OutputConfig{}

	outputConfig.Port = getConfigFieldValue("port", userConfigMap, baseRuleMap)
	outputConfig.SocksPort = getConfigFieldValue("socks-port", userConfigMap, baseRuleMap)
	outputConfig.RedirPort = getConfigFieldValue("redir-port", userConfigMap, baseRuleMap)
	outputConfig.MixedPort = getConfigFieldValue("mixed-port", userConfigMap, baseRuleMap)

	authentication := getConfigFieldValue("authentication", userConfigMap, baseRuleMap)
	if authentication != nil {
		outputConfig.Authentication = authentication.([]interface{})
	} else {
		outputConfig.Authentication = nil
	}

	outputConfig.AllowLan = getConfigFieldValue("allow-lan", userConfigMap, baseRuleMap)
	outputConfig.BindAddress = getConfigFieldValue("bind-address", userConfigMap, baseRuleMap)
	outputConfig.Mode = getConfigFieldValue("mode", userConfigMap, baseRuleMap)
	outputConfig.LogLevel = getConfigFieldValue("log-level", userConfigMap, baseRuleMap)
	outputConfig.Ipv6 = getConfigFieldValue("ipv6", userConfigMap, baseRuleMap)
	outputConfig.ExternalController = getConfigFieldValue("external-controller", userConfigMap, baseRuleMap)
	outputConfig.ExternalUi = getConfigFieldValue("external-ui", userConfigMap, baseRuleMap)
	outputConfig.Secret = getConfigFieldValue("secret", userConfigMap, baseRuleMap)
	outputConfig.InterfaceName = getConfigFieldValue("interface-name", userConfigMap, baseRuleMap)

	hosts := getConfigFieldValue("hosts", userConfigMap, baseRuleMap)
	if hosts != nil {
		outputConfig.Hosts = hosts.(map[interface{}]interface{})
	} else {
		outputConfig.Hosts = nil
	}

	dns := getConfigFieldValue("dns", userConfigMap, baseRuleMap)
	if dns != nil {
		outputConfig.Dns = dns.(map[interface{}]interface{})
	} else {
		outputConfig.Dns = nil
	}

	outputConfig.Rules = getConfigFieldMergeValueArr("rules", userConfigMap, baseRuleMap)

	outputConfig.RuleProviders = getConfigFieldMergeValueMap("rule-providers", userConfigMap, baseRuleMap)
	outputConfig.ProxyProviders = getConfigFieldMergeValueMap("proxy-providers", userConfigMap, baseRuleMap)

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
	marshalRule, _ := yaml.Marshal(&outputConfig)
	fmt.Fprintln(w, string(marshalRule))
}

// 获取配置文件，二选一的，优先返回用户设置的，其次返回基础配置的，最后返回空
func getConfigFieldValue(fieldName string, userConfigMap map[string]interface{}, baseRuleConfigMap map[string]interface{}) interface{} {
	if userConfigMap[fieldName] != nil {
		return userConfigMap[fieldName]
	} else if baseRuleConfigMap[fieldName] != nil {
		return baseRuleConfigMap[fieldName]
	} else {
		return nil
	}
	return nil
}

// 把用户自定义设置和基础配置合并后返回
func getConfigFieldMergeValueArr(fieldName string, userConfigMap map[string]interface{}, baseRuleConfigMap map[string]interface{}) []interface{} {
	var valueSlice []interface{}
	if userConfigMap[fieldName] != nil {
		valueSlice = append(valueSlice, userConfigMap[fieldName].([]interface{})...)
	}
	if baseRuleConfigMap[fieldName] != nil {
		valueSlice = append(valueSlice, baseRuleConfigMap[fieldName].([]interface{})...)
	}
	return valueSlice
}

func getConfigFieldMergeValueMap(fieldName string, userConfigMap map[string]interface{}, baseRuleConfigMap map[string]interface{}) map[interface{}]interface{} {
	valueMap := make(map[interface{}]interface{})
	if baseRuleConfigMap[fieldName] != nil {
		baseRuleConfigValueMap := baseRuleConfigMap[fieldName].(map[interface{}]interface{})
		for key := range baseRuleConfigValueMap {
			valueMap[key] = baseRuleConfigValueMap[key]
		}
	}
	if userConfigMap[fieldName] != nil {
		userConfigValueMap := userConfigMap[fieldName].(map[interface{}]interface{})
		for key := range userConfigValueMap {
			valueMap[key] = userConfigValueMap[key]
		}
	}
	return valueMap
}
