package base_rule

import (
	"parseAndCombineMyClashRules/model"
)

type Hackl0us struct {
	Rule model.Rule
}

func (hakcl0us Hackl0us) MergeRule(customConfig model.Config, proxyArr map[string]interface{}) model.Rule {
	// 合并config 参数
	if customConfig.Port > 0 {
		hakcl0us.Rule.Port = customConfig.Port
	}

	if customConfig.SocksPort > 0 {
		hakcl0us.Rule.SocksPort = customConfig.SocksPort
	}

	if customConfig.AllowLan {
		hakcl0us.Rule.AllowLan = customConfig.AllowLan
	}

	if customConfig.Mode != "" {
		hakcl0us.Rule.Mode = customConfig.Mode
	}

	if customConfig.LogLevel != "" {
		hakcl0us.Rule.LogLevel = customConfig.LogLevel
	}

	if customConfig.ExternalController != "" {
		hakcl0us.Rule.ExternalController = customConfig.ExternalController
	}

	if customConfig.ExternalUi != "" {
		hakcl0us.Rule.ExternalUi = customConfig.ExternalUi
	}

	if customConfig.Secret != "" {
		hakcl0us.Rule.ExternalUi = customConfig.Secret
	}

	if customConfig.Experimental.IgnoreResolveFail == true {
		hakcl0us.Rule.Experimental = customConfig.Experimental
	}

	if hakcl0us.Rule.FallbackFilter.GeoIp == true {
		hakcl0us.Rule.Dns.FallbackFilter = hakcl0us.Rule.FallbackFilter
	}

	if customConfig.Dns.EnableDns == true {
		hakcl0us.Rule.Dns = customConfig.Dns
	}

	if customConfig.FallbackFilter.GeoIp == true {
		hakcl0us.Rule.Dns.FallbackFilter = customConfig.FallbackFilter
	}

	if len(customConfig.CfwBypass) > 0 {
		hakcl0us.Rule.CfwBypass = customConfig.CfwBypass
	}

	if customConfig.CfwLatencyTimeout > 0 {
		hakcl0us.Rule.CfwLatencyTimeout = customConfig.CfwLatencyTimeout
	}

	if len(customConfig.Rule) > 0 {
		hakcl0us.Rule.Rule = append(customConfig.Rule, hakcl0us.Rule.Rule...)
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
	customConfig.ProxyGroup = append(customConfig.ProxyGroup, model.ProxyGroup{
		Name:    "Proxy",
		Type:    "select",
		Proxies: writeProxyName,
	})
	hakcl0us.Rule.Proxy = writeProxy
	hakcl0us.Rule.ProxyGroup = customConfig.ProxyGroup

	return hakcl0us.Rule
}
