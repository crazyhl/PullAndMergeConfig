package base_rule

import (
	"parseAndCombineMyClashRules/model"
	"strings"
)

type ConnersHua struct {
	Rule model.Rule
}

func (connersHua ConnersHua) MergeRule(customConfig model.Config, proxyArr map[string][]model.Proxy) model.Rule {
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

	connersHua.Rule.Proxy = writeProxy
	for _, customGroupInfo := range customConfig.ProxyGroup {
		connersHua.Rule.ProxyGroup = append(connersHua.Rule.ProxyGroup, customGroupInfo)
	}
	// 处理他自己的各个组
	for index, groupInfo := range connersHua.Rule.ProxyGroup {
		switch groupInfo.Name {
		case "UrlTest":
			connersHua.Rule.ProxyGroup[index].Proxies = writeProxyName
		case "PROXY":
			connersHua.Rule.ProxyGroup[index].Proxies = append([]string{"UrlTest"}, writeProxyName...)
		case "GlobalMedia":
			connersHua.Rule.ProxyGroup[index].Proxies = append([]string{"PROXY"}, writeProxyName...)
		case "HKMTMedia":
			connersHua.Rule.ProxyGroup[index].Proxies = []string{"DIRECT", "PROXY"} //append(, writeProxyName...)
			for _, proxyName := range writeProxyName {
				if strings.Contains(strings.ToLower(proxyName), "hk") ||
					strings.Contains(strings.ToLower(proxyName), "港") {
					connersHua.Rule.ProxyGroup[index].Proxies = append(connersHua.Rule.ProxyGroup[index].Proxies, proxyName)
				}
			}
		}
	}

	return connersHua.Rule
}
