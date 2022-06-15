package utils

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"net/url"
	"parseAndCombineMyClashRules/my_set"
	"strconv"
	"strings"
)

func GetBase64Decode(s string) []byte {
	decodeBase64Proxy, decodeBase64ProxyErr := base64.RawURLEncoding.DecodeString(s)
	if decodeBase64ProxyErr != nil {
		decodeBase64Proxy, decodeBase64ProxyErr = base64.RawStdEncoding.DecodeString(s)
		if decodeBase64ProxyErr != nil {
			decodeBase64Proxy, decodeBase64ProxyErr = base64.URLEncoding.DecodeString(s)
			if decodeBase64ProxyErr != nil {
				decodeBase64Proxy, decodeBase64ProxyErr = base64.StdEncoding.DecodeString(s)
				if decodeBase64ProxyErr != nil {
					return nil
				}
			}
		}
	}

	return decodeBase64Proxy
}

// 解析 base64 格式的服务器信息
func ParseBase64Proxy(proxyBody []byte, filterProxyName []interface{}, filterProxyServer []interface{}) ([]map[interface{}]interface{}, error) {
	// base64 解析失败了，尝试解析 yaml
	base64ProxyArr, parseBase64ProxyArr := parseBase64ProxyArr(proxyBody)
	if parseBase64ProxyArr != nil {
		return nil, errors.New("订阅 Proxy 信息 base64 解析失败")
	}
	filterProxyArr := filterUnAddProxyServer(base64ProxyArr, filterProxyName, filterProxyServer)

	return filterProxyArr, nil
}

// 解析 yaml 格式的服务器信息
func ParseYamlProxy(proxyBody []byte, filterProxyName []interface{}, filterProxyServer []interface{}) ([]map[interface{}]interface{}, error) {
	// base64 解析失败了，尝试解析 yaml
	proxyRule := make(map[interface{}]interface{})
	unmarshalProxyRuleErr := yaml.Unmarshal(proxyBody, &proxyRule)
	if unmarshalProxyRuleErr != nil {
		fmt.Println(string(proxyBody))
		fmt.Println(unmarshalProxyRuleErr)
		return nil, errors.New("订阅 Proxy 信息 yaml 解析失败,请检查订阅url是否正确")
	}
	var proxyServerArr []map[interface{}]interface{}
	if proxyRule["proxies"] != nil {
		for _, proxy := range proxyRule["proxies"].([]interface{}) {
			proxyMap := proxy.(map[interface{}]interface{})
			proxyServerArr = append(proxyServerArr, proxyMap)
		}
	}
	if proxyRule["Proxy"] != nil {
		for _, proxy := range proxyRule["Proxy"].([]interface{}) {
			proxyMap := proxy.(map[interface{}]interface{})
			proxyServerArr = append(proxyServerArr, proxyMap)
		}
	}

	filterProxyArr := filterUnAddProxyServer(proxyServerArr, filterProxyName, filterProxyServer)

	return filterProxyArr, nil
}

// 解析 base64 的代理数组信息
func parseBase64ProxyArr(base64ProxyStr []byte) ([]map[interface{}]interface{}, error) {
	// 把base64 换行
	proxyStrArr := strings.Split(string(base64ProxyStr), "\n")
	var proxyArr []map[interface{}]interface{}
	// 用来过滤重名测map
	proxyNameSet := my_set.MySet{}
	// 清空来做一下初始化工作
	proxyNameSet.Clear()
	// 遍历分割的base 64字符串
	for _, proxyStr := range proxyStrArr {
		proxyStr = strings.Trim(proxyStr, "\r")
		// 判断是否已vmess开头，目前仅支持vmess
		if strings.HasPrefix(proxyStr, "vmess://") {
			proxyStr = proxyStr[8:]
			vmessProxy, vmessProxyErr := base64.RawURLEncoding.DecodeString(proxyStr)

			if vmessProxyErr == nil {
				vmessProxyMap := make(map[string]interface{})
				unmarshalVmessProxyErr := json.Unmarshal(vmessProxy, &vmessProxyMap)
				if unmarshalVmessProxyErr != nil {
					fmt.Println(unmarshalVmessProxyErr)
					return nil, unmarshalVmessProxyErr
				}
				alertId := 0
				switch vmessProxyMap["aid"].(type) {
				case float64:
					alertId = int(vmessProxyMap["aid"].(float64))
				case string:
					alertId, _ = strconv.Atoi(vmessProxyMap["aid"].(string))
				}

				proxyName := vmessProxyMap["ps"].(string)
				for {
					contains := proxyNameSet.Contains(proxyName)
					if contains {
						proxyName = proxyName + "$"
					} else {
						proxyNameSet.Add(proxyName)
						break
					}
				}
				proxyMap := make(map[interface{}]interface{})
				proxyMap["name"] = proxyName
				proxyMap["type"] = "vmess"
				proxyMap["server"] = vmessProxyMap["add"]
				proxyMap["port"] = vmessProxyMap["port"]
				proxyMap["cipher"] = "auto"
				proxyMap["uuid"] = vmessProxyMap["id"]
				proxyMap["alterId"] = alertId
				if vmessProxyMap["tls"] != nil && vmessProxyMap["tls"] != "" {
					proxyMap["tls"] = true
				}
				proxyMap["network"] = vmessProxyMap["net"]
				proxyMap["ws-path"] = vmessProxyMap["path"]
				proxyWsHeaders := make(map[interface{}]interface{})
				proxyWsHeaders["Host"] = vmessProxyMap["host"]
				proxyMap["ws-headers"] = proxyWsHeaders
				proxyArr = append(proxyArr, proxyMap)
			}
		} else if strings.HasPrefix(proxyStr, "trojan://") {
			urlParseInfo, urlParseErr := url.Parse(proxyStr)
			if urlParseErr == nil {
				proxyMap := make(map[interface{}]interface{})
				proxyMap["name"] = urlParseInfo.Fragment
				proxyMap["type"] = "trojan"
				proxyMap["server"] = urlParseInfo.Hostname()
				proxyMap["port"] = urlParseInfo.Port()
				proxyMap["password"] = urlParseInfo.User.String()
				proxyMap["udp"] = true
				if sni, ok := urlParseInfo.Query()["sni"]; ok {
					proxyMap["sni"] = sni[0]
				}
				proxyArr = append(proxyArr, proxyMap)
			}
		} else if strings.HasPrefix(proxyStr, "ss://") {
			urlParseInfo, urlParseErr := url.Parse(proxyStr)
			if urlParseErr == nil {
				vmessProxy, vmessProxyErr := base64.RawURLEncoding.DecodeString(urlParseInfo.User.String())
				if vmessProxyErr == nil {
					methodAndPassword := strings.Split(string(vmessProxy), ":")
					proxyMap := make(map[interface{}]interface{})
					proxyMap["name"] = urlParseInfo.Fragment
					proxyMap["type"] = "ss"
					proxyMap["server"] = urlParseInfo.Hostname()
					proxyMap["port"] = urlParseInfo.Port()
					proxyMap["password"] = methodAndPassword[1]
					proxyMap["cipher"] = methodAndPassword[0]

					proxyArr = append(proxyArr, proxyMap)
				}

			}
		}
	}

	return proxyArr, nil
}

// 过滤不想要加入的proxy
func filterUnAddProxyServer(proxyServerArr []map[interface{}]interface{}, filterProxyName []interface{}, filterProxyServer []interface{}) []map[interface{}]interface{} {
	// 解析成功了赋值
	var proxyArr []map[interface{}]interface{}
filterStart:
	for _, proxy := range proxyServerArr {
		for _, filterName := range filterProxyName {
			if filterName == proxy["name"] {
				continue filterStart
			}
		}
		for _, server := range filterProxyServer {
			if server == proxy["server"] {
				continue filterStart
			}
		}
		proxyArr = append(proxyArr, proxy)
	}
	return proxyArr
}
