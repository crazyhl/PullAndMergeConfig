package main

import (
	"encoding/base64"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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
	userConfigMap, baseRuleMap, done := parseRequestParams(r)
	if done {
		return
	}

	proxyArr, proxyGroupArr, proxyNameArr, done2 := getProxies(userConfigMap, r)
	if done2 {
		return
	}

	outPutClash(w, userConfigMap, baseRuleMap, proxyArr, proxyGroupArr, proxyNameArr)
}

func outPutClash(w http.ResponseWriter, userConfigMap map[string]interface{}, baseRuleMap map[string]interface{}, proxyArr []map[interface{}]interface{}, proxyGroupArr []map[interface{}]interface{}, proxyNameArr []interface{}) {
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
	// proxy providers
	filterProxyProvier := userConfigMap["filter-proxy-providers"]
	var filterProxyProviderArr []interface{}
	if filterProxyProvier != nil {
		filterProxyProviderArr = filterProxyProvier.([]interface{})
	}
	outputConfig.ProxyProviders = getConfigFieldMergeValueMap("proxy-providers", userConfigMap, baseRuleMap)
	for providerName := range outputConfig.ProxyProviders {
		for _, proxyProvider := range filterProxyProviderArr {
			if providerName == proxyProvider {
				delete(outputConfig.ProxyProviders, providerName)
			}
		}
	}
	// 构造 proxy
	outputConfig.Proxies = proxyArr
	// 构造 proxyGroup
	filterProxyGroup := userConfigMap["filter-proxy-groups"]
	var filterProxyGroupArr []interface{}
	if filterProxyGroup != nil {
		filterProxyGroupArr = filterProxyGroup.([]interface{})
	}
	var outputProxyGroupMap []map[interface{}]interface{}
	// 加入上面几组订阅的单独group
	outputProxyGroupMap = append(outputProxyGroupMap, proxyGroupArr...)
	userConfigProxyGroups := userConfigMap["proxy-groups"]
	if userConfigProxyGroups != nil {
		userConfigProxyGroupMapArr := generateProxyNameToGroup(userConfigProxyGroups, proxyNameArr, filterProxyGroupArr)
		outputProxyGroupMap = append(outputProxyGroupMap, userConfigProxyGroupMapArr...)
	}
	baseRuleProxyGroups := baseRuleMap["proxy-groups"]
	if baseRuleProxyGroups != nil {
		baseRuleProxyGroupMapArr := generateProxyNameToGroup(baseRuleProxyGroups.([]interface{}), proxyNameArr, filterProxyGroupArr)
		// 重新整理一下需要把一些 group name 加入到 select 组中
		var noneSelectGroupName []interface{}
		for _, proxyGroupMap := range baseRuleProxyGroupMapArr {
			if proxyGroupMap["type"] != "select" {
				noneSelectGroupName = append(noneSelectGroupName, proxyGroupMap["name"])
			}
		}
		for idx, proxyGroupMap := range baseRuleProxyGroupMapArr {
			if proxyGroupMap["type"] == "select" {
				baseRuleProxyGroupMapArr[idx]["proxies"] = append(noneSelectGroupName, baseRuleProxyGroupMapArr[idx]["proxies"].([]interface{})...)
			}
		}
		outputProxyGroupMap = append(outputProxyGroupMap, baseRuleProxyGroupMapArr...)
	}

	outputConfig.ProxyGroups = outputProxyGroupMap

	marshalRule, _ := yaml.Marshal(&outputConfig)
	fmt.Fprintln(w, string(marshalRule))
}

func getProxies(userConfigMap map[string]interface{}, r *http.Request) ([]map[interface{}]interface{}, []map[interface{}]interface{}, []interface{}, bool) {
	// 获取配置文件输入的订阅地址
	pullProxySource := userConfigMap["pull-proxy-source"]
	if pullProxySource == nil {
		return nil, nil, nil, true
	}
	pullProxySourceMap := pullProxySource.([]interface{})
	pullProxySourceMapLength := len(pullProxySourceMap)
	if pullProxySourceMapLength <= 0 {
		log.Printf("请求url: %s，请求IP: %s\n，没有获取到订阅地址", r.URL, utils.GetRequestIp(r))
	}
	// 多线程获取订阅
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(pullProxySourceMapLength)
	var proxyArr []map[interface{}]interface{}
	var proxyGroupArr []map[interface{}]interface{}
	var proxyNameArr []interface{}
	for _, proxySource := range pullProxySourceMap {
		go func(proxySource interface{}) {
			defer waitGroup.Done()
			proxySourceMap := proxySource.(map[interface{}]interface{})
			// 获取订阅
			httpResponseBytes, httpRequestErr := utils.HttpGet(proxySourceMap["url"].(string))
			if httpRequestErr != nil {
				log.Printf(
					"请求url: %s，请求IP: %s, 订阅地址: %s, 报错内容：%v\n，通过订阅地址获取内容失败",
					r.URL,
					utils.GetRequestIp(r),
					proxySourceMap["url"].(string),
					httpRequestErr.Error(),
				)
			}
			filterProxyName := userConfigMap["filter-proxy-name"]
			var filterProxyNameArr []interface{}
			if filterProxyName != nil {
				filterProxyNameArr = filterProxyName.([]interface{})
			}

			filterProxyServer := userConfigMap["filter-proxy-server"]
			var filterProxyServerArr []interface{}
			if filterProxyServer != nil {
				filterProxyServerArr = filterProxyServer.([]interface{})
			}

			decodeBase64Proxy, decodeBase64ProxyErr := base64.URLEncoding.DecodeString(string(httpResponseBytes))
			if decodeBase64ProxyErr != nil {
				// 在使用另一个base64 解析
				decodeStdBase64Proxy, decodeStdBase64ProxyErr := base64.StdEncoding.DecodeString(string(httpResponseBytes))
				if decodeStdBase64ProxyErr != nil {
					yamlProxyServerArr, yamlProxyServerErr := utils.ParseYamlProxy(
						httpResponseBytes,
						filterProxyNameArr,
						filterProxyServerArr,
					)
					if yamlProxyServerErr != nil {
						log.Printf(
							"请求url: %s，请求IP: %s, 订阅地址: %s\n，解析base64 和 yaml 全部失败",
							r.URL,
							utils.GetRequestIp(r),
							proxySourceMap["url"].(string),
						)
					}
					proxyGroupMap, tempProxyNameArr := generateGroupAndProxyNameArr(yamlProxyServerArr, proxySourceMap["name"])
					proxyGroupArr = append(proxyGroupArr, proxyGroupMap)
					proxyArr = append(proxyArr, yamlProxyServerArr...)
					proxyNameArr = append(proxyNameArr, tempProxyNameArr...)
					return
				}
				// 不是 base64 ,解析yaml
				base64ProxyServerArr, base64ProxyServerErr := utils.ParseBase64Proxy(
					decodeStdBase64Proxy,
					filterProxyNameArr,
					filterProxyServerArr,
				)
				if base64ProxyServerErr != nil {
					return
				}

				proxyGroupMap, tempProxyNameArr := generateGroupAndProxyNameArr(base64ProxyServerArr, proxySourceMap["name"])
				proxyGroupArr = append(proxyGroupArr, proxyGroupMap)
				proxyArr = append(proxyArr, base64ProxyServerArr...)
				proxyNameArr = append(proxyNameArr, tempProxyNameArr...)
				return
			}
			//_, _ = hiMagenta.Println(source.Name + "订阅 Proxy 信息是 base64 文件，尝试用 base64 解析")
			base64ProxyServerArr, base64ProxyServerErr := utils.ParseBase64Proxy(
				decodeBase64Proxy,
				filterProxyNameArr,
				filterProxyServerArr,
			)
			if base64ProxyServerErr != nil {
				return
			}

			proxyGroupMap, tempProxyNameArr := generateGroupAndProxyNameArr(base64ProxyServerArr, proxySourceMap["name"])
			proxyGroupArr = append(proxyGroupArr, proxyGroupMap)
			proxyArr = append(proxyArr, base64ProxyServerArr...)
			proxyNameArr = append(proxyNameArr, tempProxyNameArr...)
		}(proxySource)
	}
	waitGroup.Wait()
	return proxyArr, proxyGroupArr, proxyNameArr, false
}

func parseRequestParams(r *http.Request) (map[string]interface{}, map[string]interface{}, bool) {
	// 获取配置参数
	configFileNameArr, getConfigFileNameOk := r.URL.Query()["name"]
	if getConfigFileNameOk == false {
		return nil, nil, true
	}
	configFileName := configFileNameArr[0]
	configFile, readConfigFileErr := ioutil.ReadFile(absPath + "/config/" + configFileName + ".yaml")
	if readConfigFileErr != nil {
		log.Printf("error: %v\n", readConfigFileErr)
		log.Println("读入配置文件失败了，检查配置文件是否存在")
		return nil, nil, true
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
		return nil, nil, true
	}

	if baseConfigRequestUrl == "" {
		log.Printf("请求url: %s，请求IP: %s\n，没有找到对应配置文件", r.URL, utils.GetRequestIp(r))
	}

	// 拉取基础配置
	baseRuleBody, baseRuleErr := utils.HttpGet(baseConfigRequestUrl)
	if baseRuleErr != nil {
		log.Println(baseRuleErr)
		log.Printf("请求url: %s，请求IP: %s\n，拉取基础规则失败，请检查网络！！！！", r.URL, utils.GetRequestIp(r))
		return nil, nil, true
	}

	baseRuleMap := make(map[string]interface{})
	unmarshalReadyToWriteRuleErr := yaml.Unmarshal(baseRuleBody, &baseRuleMap)
	if unmarshalReadyToWriteRuleErr != nil {
		log.Printf("error: %v\n", unmarshalReadyToWriteRuleErr)
		log.Println("解析基础规则失败，请检查 url 是否正确 ！！！")
		return nil, nil, true
	}
	return userConfigMap, baseRuleMap, false
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

// 生成 proxyGroup 并且返回该组的 proxy 名称
func generateGroupAndProxyNameArr(base64ProxyServerArr []map[interface{}]interface{}, proxySourceName interface{}) (map[interface{}]interface{}, []interface{}) {
	var tempProxyNameArr []interface{}
	for mapIndex := range base64ProxyServerArr {
		tempProxyNameArr = append(tempProxyNameArr, base64ProxyServerArr[mapIndex]["name"])
	}
	// 构造一个 group
	proxyGroupMap := make(map[interface{}]interface{})
	proxyGroupMap["name"] = proxySourceName
	proxyGroupMap["type"] = "select"
	proxyGroupMap["proxies"] = tempProxyNameArr

	return proxyGroupMap, tempProxyNameArr
}

// 把 ProxyName数组 插入到 group 中
func generateProxyNameToGroup(proxyGroups interface{}, proxyNameArr []interface{}, filterProxyGroupArr []interface{}) []map[interface{}]interface{} {
	// 遍历 判定是否包含use 如果没有就插入 proxies
	var outputProxyGroupMap []map[interface{}]interface{}
filterStart:
	for _, proxyGroup := range proxyGroups.([]interface{}) {
		proxyGroupMap := proxyGroup.(map[interface{}]interface{})
		for _, filterName := range filterProxyGroupArr {
			if filterName == proxyGroupMap["name"] {
				continue filterStart
			}
		}

		if proxyGroupMap["use"] == nil {
			proxyGroupMap["proxies"] = proxyNameArr
		}

		outputProxyGroupMap = append(outputProxyGroupMap, proxyGroupMap)
	}
	return outputProxyGroupMap
}
