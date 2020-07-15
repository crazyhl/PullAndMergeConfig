package base_rule

import (
	"fmt"
	"parseAndCombineMyClashRules/model"
	"strings"
)

type ConnersHua struct {
	Rule model.Rule
}

func (connersHua ConnersHua) MergeRule(customConfig model.Config, proxyArr map[string]interface{}) model.Rule {
	// 合并config 参数
	if customConfig.Port > 0 {
		connersHua.Rule.Port = customConfig.Port
	}

	if customConfig.SocksPort > 0 {
		connersHua.Rule.SocksPort = customConfig.SocksPort
	}

	if customConfig.AllowLan {
		connersHua.Rule.AllowLan = customConfig.AllowLan
	}

	if customConfig.Mode != "" {
		connersHua.Rule.Mode = customConfig.Mode
	}

	if customConfig.LogLevel != "" {
		connersHua.Rule.LogLevel = customConfig.LogLevel
	}

	if customConfig.ExternalController != "" {
		connersHua.Rule.ExternalController = customConfig.ExternalController
	}

	if customConfig.ExternalUi != "" {
		connersHua.Rule.ExternalUi = customConfig.ExternalUi
	}

	if customConfig.Secret != "" {
		connersHua.Rule.ExternalUi = customConfig.Secret
	}

	if customConfig.Experimental.IgnoreResolveFail == true {
		connersHua.Rule.Experimental = customConfig.Experimental
	}

	if connersHua.Rule.FallbackFilter.GeoIp == true {
		connersHua.Rule.Dns.FallbackFilter = connersHua.Rule.FallbackFilter
	}

	if customConfig.Dns.EnableDns == true {
		connersHua.Rule.Dns = customConfig.Dns
	}

	if customConfig.FallbackFilter.GeoIp == true {
		connersHua.Rule.Dns.FallbackFilter = customConfig.FallbackFilter
	}

	if len(customConfig.CfwBypass) > 0 {
		connersHua.Rule.CfwBypass = customConfig.CfwBypass
	}

	if customConfig.CfwLatencyTimeout > 0 {
		connersHua.Rule.CfwLatencyTimeout = customConfig.CfwLatencyTimeout
	}

	if len(customConfig.Rule) > 0 {
		connersHua.Rule.Rule = append(customConfig.Rule, connersHua.Rule.Rule...)
	}
	if len(customConfig.Rules) > 0 {
		connersHua.Rule.Rules = append(customConfig.Rules, connersHua.Rule.Rules...)
	}
	// 合并 rule
	connersHua.Rule.Rules = append(connersHua.Rule.Rules, connersHua.Rule.Rule...)
	connersHua.Rule.Rule = nil

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
		if proxier != nil {
			proxier, ok := proxier.([]model.Proxy)
			if ok {
				for _, p := range proxier {
					writeProxyName = append(writeProxyName, p.Name)
					writeProxy = append(writeProxy, p)
				}
			}
		}
	}

	if len(customConfig.ProxyGroup) > 0 {
		for index, pGroup := range customConfig.ProxyGroup {
			writeProxyGroupItemNameArr = append(writeProxyGroupItemNameArr, pGroup.Name)
			customConfig.ProxyGroup[index].Proxies = writeProxyName
		}
	}
	writeProxyName = append(writeProxyGroupItemNameArr, writeProxyName...)
	fmt.Println(connersHua)

	connersHua.Rule.Proxies = writeProxy
	connersHua.Rule.Proxy = nil
	for _, customGroupInfo := range customConfig.ProxyGroup {
		connersHua.Rule.ProxyGroups = append(connersHua.Rule.ProxyGroups, customGroupInfo)
	}
	//connersHua.Rule.ProxyGroups = connersHua.Rule.ProxyGroup
	connersHua.Rule.ProxyGroup = nil
	// 处理他自己的各个组
	for index, groupInfo := range connersHua.Rule.ProxyGroups {
		switch groupInfo.Name {
		case "UrlTest":
			connersHua.Rule.ProxyGroups[index].Proxies = writeProxyName
		case "PROXY":
			connersHua.Rule.ProxyGroups[index].Proxies = append([]string{"UrlTest"}, writeProxyName...)
		case "GlobalMedia":
			connersHua.Rule.ProxyGroups[index].Proxies = append([]string{"PROXY"}, writeProxyName...)
		case "HKMTMedia":
			connersHua.Rule.ProxyGroups[index].Proxies = []string{"DIRECT", "PROXY"} //append(, writeProxyName...)
			for _, proxyName := range writeProxyName {
				if strings.Contains(strings.ToLower(proxyName), "hk") ||
					strings.Contains(strings.ToLower(proxyName), "港") {
					connersHua.Rule.ProxyGroups[index].Proxies = append(connersHua.Rule.ProxyGroups[index].Proxies, proxyName)
				}
			}
		}
	}

	return connersHua.Rule
}
