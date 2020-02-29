package main

import (
	"encoding/base64"
	"encoding/json"
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
	//	vmessStr := "123"
	//	vmessByteArr, err := base64.URLEncoding.DecodeString(vmessStr)
	//	fmt.Println(string(vmessByteArr))
	//	fmt.Println("---" + err.Error() + "------")
	// 定义一波颜色
	hiYellowColor := color.New(color.FgHiYellow)
	hiRedColor := color.New(color.FgHiRed)
	cyanColor := color.New(color.FgCyan)
	hiMagenta := color.New(color.FgHiMagenta)
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
	hiYellowColor.Println("开始拉取订阅代理信息...")
	pullProxySourceCunt := len(parseConfig.PullProxySource)
	wg := sync.WaitGroup{}
	wg.Add(pullProxySourceCunt)
	proxyArr := make(map[string][]model.Proxy)
	for i := 0; i < pullProxySourceCunt; i++ {
		go func(source model.PullProxySource) {
			defer wg.Done()
			proxyArr[source.Name] = nil
			proxyBody, proxyErr := httpGet(source.Url)
			if baseRuleErr != nil {
				hiRedColor.Println(proxyErr)
				hiRedColor.Println(source.Name + "获取节点信息失败，请检查网络！！！")
				return
			}

			decodeProxy, decodeProxyErr := base64.URLEncoding.DecodeString(string(proxyBody))
			if decodeProxyErr != nil {
				// base64 解析失败了，尝试解析 yaml
				hiMagenta.Println(source.Name + "订阅不是 base64 文件，尝试用yaml解析")
				proxyRule := model.Rule{}
				err = yaml.Unmarshal(proxyBody, &proxyRule)
				if err != nil {
					hiRedColor.Println(source.Name + "订阅 yaml 也解析失败了")
					return
				}
				// 解析成功了赋值
				proxyArr[source.Name] = proxyRule.Proxy
				hiMagenta.Println(source.Name + "获取节点信息成功，是 yaml 格式的。")
				return
			}
			base64DecodeProxyArr := strings.Split(string(decodeProxy), "\n")
			var base64ProxyArr []model.Proxy
			for _, proxyStr := range base64DecodeProxyArr {
				if strings.HasPrefix(proxyStr, "vmess://") {
					proxyStr = proxyStr[8:]
					vmessProxy, vmessProxyErr := base64.URLEncoding.DecodeString(proxyStr)

					if vmessProxyErr == nil {
						vmessProxyModel := model.Base64VmessProxy{}
						err = json.Unmarshal(vmessProxy, &vmessProxyModel)
						if err != nil {
							hiRedColor.Println(err)
							hiRedColor.Println(source.Name + " vmess 解析失败了")
							return
						}
						alertId, _ := strconv.Atoi(vmessProxyModel.AlterId)

						base64ProxyArr = append(base64ProxyArr, model.Proxy{
							Name:           vmessProxyModel.Name,
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
			proxyArr[source.Name] = base64ProxyArr

		}(parseConfig.PullProxySource[i])
	}
	wg.Wait()
	hiYellowColor.Println("开始合并配置文件...")
	// 写出规则文件
	parseRule := model.Rule{}
	err = yaml.Unmarshal(baseRuleBody, &parseRule)
	if err != nil {
		hiRedColor.Printf("error: %v\n", err)
		return
	}
	// 合并config 参数
	if parseConfig.Port > 0 {
		parseRule.Port = parseConfig.Port
	}

	if parseConfig.SocksPort > 0 {
		parseRule.SocksPort = parseConfig.SocksPort
	}

	if parseConfig.AllowLan {
		parseRule.AllowLan = parseConfig.AllowLan
	}

	if parseConfig.Mode != "" {
		parseRule.Mode = parseConfig.Mode
	}

	if parseConfig.LogLevel != "" {
		parseRule.LogLevel = parseConfig.LogLevel
	}

	if parseConfig.ExternalController != "" {
		parseRule.ExternalController = parseConfig.ExternalController
	}

	if parseConfig.ExternalUi != "" {
		parseRule.ExternalUi = parseConfig.ExternalUi
	}

	if parseConfig.Secret != "" {
		parseRule.ExternalUi = parseConfig.Secret
	}

	if parseConfig.Experimental.IgnoreResolveFail == true {
		parseRule.Experimental = parseConfig.Experimental
	}

	if parseRule.FallbackFilter.GeoIp == true {
		parseRule.Dns.FallbackFilter = parseRule.FallbackFilter
	}

	if parseConfig.Dns.EnableDns == true {
		parseRule.Dns = parseConfig.Dns
	}

	if parseConfig.FallbackFilter.GeoIp == true {
		parseRule.Dns.FallbackFilter = parseConfig.FallbackFilter
	}

	if len(parseConfig.CfwBypass) > 0 {
		parseRule.CfwBypass = parseConfig.CfwBypass
	}

	if parseConfig.CfwLatencyTimeout > 0 {
		parseRule.CfwLatencyTimeout = parseConfig.CfwLatencyTimeout
	}

	if len(parseConfig.Rule) > 0 {
		parseRule.Rule = append(parseRule.Rule, parseRule.Rule...)
	}

	var writeProxyGroupItemNameArr []string
	var writeProxyName []string

	var writeProxy []model.Proxy
	for _, proxier := range proxyArr {
		for _, p := range proxier {
			writeProxyName = append(writeProxyName, p.Name)
			writeProxy = append(writeProxy, p)
		}
	}

	if len(parseConfig.ProxyGroup) > 0 {
		for index, pGroup := range parseConfig.ProxyGroup {
			writeProxyGroupItemNameArr = append(writeProxyGroupItemNameArr, pGroup.Name)
			parseConfig.ProxyGroup[index].Proxies = writeProxyName
		}
	}
	writeProxyName = append(writeProxyGroupItemNameArr, writeProxyName...)
	parseConfig.ProxyGroup = append(parseConfig.ProxyGroup, model.ProxyGroup{
		Name:    "Proxy",
		Type:    "select",
		Proxies: writeProxyName,
	})
	parseRule.Proxy = writeProxy
	parseRule.ProxyGroup = parseConfig.ProxyGroup

	d, _ := yaml.Marshal(&parseRule)

	f, err := os.Create("./" + getRule.Name + ".yaml")
	defer f.Close()
	if err != nil {
		hiRedColor.Println(err.Error())
	} else {
		_, err = f.Write(d)

	}
	//fmt.Printf("--- m dump:\n%s\n\n", string(d))
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
