package utils

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"parseAndCombineMyClashRules/model"
	"strconv"
	"strings"
)

// 解析 base64 格式的服务器信息
func ParseBase64Proxy(proxyBody []byte, filterProxyName []string, filterProxyServer []string) ([]model.Proxy, error) {
	// base64 解析失败了，尝试解析 yaml
	base64ProxyArr, parseBase64ProxyArr := parseBase64ProxyArr(proxyBody)
	if parseBase64ProxyArr != nil {
		return nil, errors.New("订阅 Proxy 信息 base64 解析失败")
	}
	filterProxyArr := filterUnAddProxyServer(base64ProxyArr, filterProxyName, filterProxyServer)

	return filterProxyArr, nil
}

// 解析 yaml 格式的服务器信息
func ParseYamlProxy(proxyBody []byte, filterProxyName []string, filterProxyServer []string) ([]model.Proxy, error) {
	// base64 解析失败了，尝试解析 yaml
	proxyRule := model.Rule{}
	unmarshalProxyRuleErr := yaml.Unmarshal(proxyBody, &proxyRule)
	if unmarshalProxyRuleErr != nil {
		fmt.Println(string(proxyBody))
		fmt.Println(unmarshalProxyRuleErr)
		return nil, errors.New("订阅 Proxy 信息 yaml 解析失败,请检查订阅url是否正确")
	}

	proxyServerArr := proxyRule.Proxy
	proxyServerArr = append(proxyServerArr, proxyRule.Proxies...)
	filterProxyArr := filterUnAddProxyServer(proxyServerArr, filterProxyName, filterProxyServer)

	return filterProxyArr, nil
}

// 解析 base64 的代理数组信息
func parseBase64ProxyArr(base64ProxyStr []byte) ([]model.Proxy, error) {
	// 把base64 换行
	proxyStrArr := strings.Split(string(base64ProxyStr), "\n")
	var proxyArr []model.Proxy
	// 用来过滤重名测map
	proxyNameSet := model.MySet{}
	// 清空来做一下初始化工作
	proxyNameSet.Clear()
	// 遍历分割的base 64字符串
	for _, proxyStr := range proxyStrArr {
		// 判断是否已vmess开头，目前仅支持vmess
		if strings.HasPrefix(proxyStr, "vmess://") {
			proxyStr = proxyStr[8:]
			vmessProxy, vmessProxyErr := base64.URLEncoding.DecodeString(proxyStr)

			if vmessProxyErr == nil {
				vmessProxyModel := model.Base64VmessProxy{}
				unmarshalVmessProxyErr := json.Unmarshal(vmessProxy, &vmessProxyModel)
				if unmarshalVmessProxyErr != nil {
					return nil, unmarshalVmessProxyErr
				}
				alertId, _ := strconv.Atoi(vmessProxyModel.AlterId)
				proxyName := vmessProxyModel.Name
				for {
					contains := proxyNameSet.Contains(proxyName)
					if contains {
						proxyName = proxyName + "$"
					} else {
						proxyNameSet.Add(proxyName)
						break
					}
				}
				proxyNameSet.Contains(proxyName)
				proxyArr = append(proxyArr, model.Proxy{
					Name:           proxyName,
					Type:           "vmess",
					Server:         vmessProxyModel.Server,
					Port:           vmessProxyModel.Port,
					Cipher:         "auto",
					Uuid:           vmessProxyModel.Uuid,
					AlterId:        alertId,
					Tls:            vmessProxyModel.Tls != "",
					SkipCertVerify: vmessProxyModel.Tls != "",
					Network:        vmessProxyModel.Network,
					WsPath:         vmessProxyModel.WsPath,
					WsHeaders: yaml.MapSlice{
						yaml.MapItem{
							Key:   "Host",
							Value: vmessProxyModel.Host,
						},
					},
				})
			}
		}
	}

	return proxyArr, nil
}

// 过滤不想要加入的proxy
func filterUnAddProxyServer(proxyServerArr []model.Proxy, filterProxyName []string, filterProxyServer []string) []model.Proxy {
	// 解析成功了赋值
	var proxyArr []model.Proxy
filterStart:
	for _, proxy := range proxyServerArr {
		for _, filterName := range filterProxyName {
			if filterName == proxy.Name {
				continue filterStart
			}
		}
		for _, server := range filterProxyServer {
			if server == proxy.Server {
				continue filterStart
			}
		}
		proxyArr = append(proxyArr, proxy)
	}
	return proxyArr
}
