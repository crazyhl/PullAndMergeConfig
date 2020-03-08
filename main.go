package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
	"parseAndCombineMyClashRules/model"
	"parseAndCombineMyClashRules/my_interface"
	"strconv"
	"strings"
	"sync"
)

func main() {
	// 定义输出颜色
	hiYellowColor := color.New(color.FgHiYellow)
	hiRedColor := color.New(color.FgHiRed)
	cyanColor := color.New(color.FgCyan)
	hiMagenta := color.New(color.FgHiMagenta)
	// 读入配置文件
	_, _ = hiYellowColor.Println("开始加载配置文件...")
	configFile, readConfigFileErr := ioutil.ReadFile("./config.yaml")
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
		rule, err := getItems(baseRules, inputArgBaseRuleName)
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
	baseRuleBody, baseRuleErr := httpGet(configBaseRule.Url)
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
			proxyBody, proxyErr := httpGet(source.Url)
			if baseRuleErr != nil {
				_, _ = hiRedColor.Println(proxyErr)
				_, _ = hiRedColor.Println(source.Name + "获取订阅 Proxy 信息，请检查网络！！！")
				return
			}

			decodeProxy, decodeProxyErr := base64.URLEncoding.DecodeString(string(proxyBody))
			if decodeProxyErr != nil {
				_, _ = hiMagenta.Println(source.Name + "订阅 Proxy 信息不是 base64 文件，尝试用yaml解析")
				yamlProxyArr, yamlProxyErr := parseYamlProxy(proxyBody, customConfig.FilterProxyName, customConfig.FilterProxyServer)
				if yamlProxyErr != nil {
					_, _ = hiMagenta.Println(source.Name + yamlProxyErr.Error())
					return
				}
				proxyArr[source.Name] = yamlProxyArr
				_, _ = hiMagenta.Println(source.Name + "获取节点信息成功， yaml 格式。")
				return
			}
			_, _ = hiMagenta.Println(source.Name + "订阅 Proxy 信息是 base64 文件，尝试用 base64 解析")
			base64ProxyServerArr, base64ProxyServerErr := parseBase64Proxy(decodeProxy, customConfig.FilterProxyName, customConfig.FilterProxyServer)
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
		readyToWriteRule = mergeHackl0usRule(customConfig, readyToWriteRule, proxyArr)
	} else if configBaseRule.Name == "ConnersHua" {
		readyToWriteRule = mergeConnersHuaRule(customConfig, readyToWriteRule, proxyArr)
	}

	marshalRule, _ := yaml.Marshal(&readyToWriteRule)

	f, err := os.Create("./" + configBaseRule.Name + ".yaml")
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
		}
	}
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

// 解析 yaml 格式的服务器信息
func parseYamlProxy(proxyBody []byte, filterProxyName []string, filterProxyServer []string) ([]model.Proxy, error) {
	// base64 解析失败了，尝试解析 yaml
	proxyRule := model.Rule{}
	unmarshalProxyRuleErr := yaml.Unmarshal(proxyBody, &proxyRule)
	if unmarshalProxyRuleErr != nil {
		fmt.Println(string(proxyBody))
		fmt.Println(unmarshalProxyRuleErr)
		return nil, errors.New("订阅 Proxy 信息 yaml 解析失败,请检查订阅url是否正确")
	}

	proxyServerArr := proxyRule.Proxy
	filterProxyArr := filterUnAddProxyServer(proxyServerArr, filterProxyName, filterProxyServer)

	return filterProxyArr, nil
}

// 解析 base64 格式的服务器信息
func parseBase64Proxy(proxyBody []byte, filterProxyName []string, filterProxyServer []string) ([]model.Proxy, error) {
	// base64 解析失败了，尝试解析 yaml
	base64ProxyArr, parseBase64ProxyArr := parseBase64ProxyArr(proxyBody)
	if parseBase64ProxyArr != nil {
		return nil, errors.New("订阅 Proxy 信息 base64 解析失败")
	}
	filterProxyArr := filterUnAddProxyServer(base64ProxyArr, filterProxyName, filterProxyServer)

	return filterProxyArr, nil
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

// 写出 Hackl0us 规则
func mergeHackl0usRule(customConfig model.Config, readyToWriteRule model.Rule, proxyArr map[string][]model.Proxy) model.Rule {
	// 合并config 参数
	if customConfig.Port > 0 {
		readyToWriteRule.Port = customConfig.Port
	}

	if customConfig.SocksPort > 0 {
		readyToWriteRule.SocksPort = customConfig.SocksPort
	}

	if customConfig.AllowLan {
		readyToWriteRule.AllowLan = customConfig.AllowLan
	}

	if customConfig.Mode != "" {
		readyToWriteRule.Mode = customConfig.Mode
	}

	if customConfig.LogLevel != "" {
		readyToWriteRule.LogLevel = customConfig.LogLevel
	}

	if customConfig.ExternalController != "" {
		readyToWriteRule.ExternalController = customConfig.ExternalController
	}

	if customConfig.ExternalUi != "" {
		readyToWriteRule.ExternalUi = customConfig.ExternalUi
	}

	if customConfig.Secret != "" {
		readyToWriteRule.ExternalUi = customConfig.Secret
	}

	if customConfig.Experimental.IgnoreResolveFail == true {
		readyToWriteRule.Experimental = customConfig.Experimental
	}

	if readyToWriteRule.FallbackFilter.GeoIp == true {
		readyToWriteRule.Dns.FallbackFilter = readyToWriteRule.FallbackFilter
	}

	if customConfig.Dns.EnableDns == true {
		readyToWriteRule.Dns = customConfig.Dns
	}

	if customConfig.FallbackFilter.GeoIp == true {
		readyToWriteRule.Dns.FallbackFilter = customConfig.FallbackFilter
	}

	if len(customConfig.CfwBypass) > 0 {
		readyToWriteRule.CfwBypass = customConfig.CfwBypass
	}

	if customConfig.CfwLatencyTimeout > 0 {
		readyToWriteRule.CfwLatencyTimeout = customConfig.CfwLatencyTimeout
	}

	if len(customConfig.Rule) > 0 {
		readyToWriteRule.Rule = append(customConfig.Rule, readyToWriteRule.Rule...)
	}

	var writeProxyGroupItemNameArr []string
	var writeProxyName []string
	var writeProxy []model.Proxy

	if len(customConfig.Proxy) > 0 {
		for _, p := range customConfig.Proxy {
			writeProxyName = append(writeProxyName, p.Name)
			writeProxy = append(writeProxy, p)
		}
	}

	for _, proxier := range proxyArr {
		for _, p := range proxier {
			writeProxyName = append(writeProxyName, p.Name)
			writeProxy = append(writeProxy, p)
		}
	}

	if len(customConfig.ProxyGroup) > 0 {
		for index, pGroup := range customConfig.ProxyGroup {
			writeProxyGroupItemNameArr = append(writeProxyGroupItemNameArr, pGroup.Name)
			customConfig.ProxyGroup[index].Proxies = writeProxyName
		}
	}
	writeProxyName = append(writeProxyGroupItemNameArr, writeProxyName...)
	customConfig.ProxyGroup = append(customConfig.ProxyGroup, model.ProxyGroup{
		Name:    "Proxy",
		Type:    "select",
		Proxies: writeProxyName,
	})
	readyToWriteRule.Proxy = writeProxy
	readyToWriteRule.ProxyGroup = customConfig.ProxyGroup

	return readyToWriteRule
}

// 写出 ConnersHua 规则
func mergeConnersHuaRule(customConfig model.Config, readyToWriteRule model.Rule, proxyArr map[string][]model.Proxy) model.Rule {
	// 合并config 参数
	if customConfig.Port > 0 {
		readyToWriteRule.Port = customConfig.Port
	}

	if customConfig.SocksPort > 0 {
		readyToWriteRule.SocksPort = customConfig.SocksPort
	}

	if customConfig.AllowLan {
		readyToWriteRule.AllowLan = customConfig.AllowLan
	}

	if customConfig.Mode != "" {
		readyToWriteRule.Mode = customConfig.Mode
	}

	if customConfig.LogLevel != "" {
		readyToWriteRule.LogLevel = customConfig.LogLevel
	}

	if customConfig.ExternalController != "" {
		readyToWriteRule.ExternalController = customConfig.ExternalController
	}

	if customConfig.ExternalUi != "" {
		readyToWriteRule.ExternalUi = customConfig.ExternalUi
	}

	if customConfig.Secret != "" {
		readyToWriteRule.ExternalUi = customConfig.Secret
	}

	if customConfig.Experimental.IgnoreResolveFail == true {
		readyToWriteRule.Experimental = customConfig.Experimental
	}

	if readyToWriteRule.FallbackFilter.GeoIp == true {
		readyToWriteRule.Dns.FallbackFilter = readyToWriteRule.FallbackFilter
	}

	if customConfig.Dns.EnableDns == true {
		readyToWriteRule.Dns = customConfig.Dns
	}

	if customConfig.FallbackFilter.GeoIp == true {
		readyToWriteRule.Dns.FallbackFilter = customConfig.FallbackFilter
	}

	if len(customConfig.CfwBypass) > 0 {
		readyToWriteRule.CfwBypass = customConfig.CfwBypass
	}

	if customConfig.CfwLatencyTimeout > 0 {
		readyToWriteRule.CfwLatencyTimeout = customConfig.CfwLatencyTimeout
	}

	if len(customConfig.Rule) > 0 {
		readyToWriteRule.Rule = append(customConfig.Rule, readyToWriteRule.Rule...)
	}

	var writeProxyGroupItemNameArr []string
	var writeProxyName []string
	var writeProxy []model.Proxy

	if len(customConfig.Proxy) > 0 {
		for _, p := range customConfig.Proxy {
			writeProxyName = append(writeProxyName, p.Name)
			writeProxy = append(writeProxy, p)
		}
	}

	for _, proxier := range proxyArr {
		for _, p := range proxier {
			writeProxyName = append(writeProxyName, p.Name)
			writeProxy = append(writeProxy, p)
		}
	}

	if len(customConfig.ProxyGroup) > 0 {
		for index, pGroup := range customConfig.ProxyGroup {
			writeProxyGroupItemNameArr = append(writeProxyGroupItemNameArr, pGroup.Name)
			customConfig.ProxyGroup[index].Proxies = writeProxyName
		}
	}
	writeProxyName = append(writeProxyGroupItemNameArr, writeProxyName...)

	readyToWriteRule.Proxy = writeProxy
	for _, customGroupInfo := range customConfig.ProxyGroup {
		readyToWriteRule.ProxyGroup = append(readyToWriteRule.ProxyGroup, customGroupInfo)
	}
	// 处理他自己的各个组
	for index, groupInfo := range readyToWriteRule.ProxyGroup {
		switch groupInfo.Name {
		case "UrlTest":
			readyToWriteRule.ProxyGroup[index].Proxies = writeProxyName
		case "PROXY":
			readyToWriteRule.ProxyGroup[index].Proxies = append([]string{"UrlTest"}, writeProxyName...)
		case "GlobalMedia":
			readyToWriteRule.ProxyGroup[index].Proxies = append([]string{"PROXY"}, writeProxyName...)
		case "HKMTMedia":
			readyToWriteRule.ProxyGroup[index].Proxies = []string{"DIRECT", "PROXY"} //append(, writeProxyName...)
			for _, proxyName := range writeProxyName {
				if strings.Contains(strings.ToLower(proxyName), "hk") ||
					strings.Contains(strings.ToLower(proxyName), "港") {
					readyToWriteRule.ProxyGroup[index].Proxies = append(readyToWriteRule.ProxyGroup[index].Proxies, proxyName)
				}
			}
		}
	}

	return readyToWriteRule
}
