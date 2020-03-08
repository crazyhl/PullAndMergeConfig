package my_interface

import "parseAndCombineMyClashRules/model"

type MergeRuleInterface interface {
	MergeRule(customConfig model.Config, proxyArr map[string][]model.Proxy) model.Rule
}

func MergeBaseRule(mergeRuleInterface MergeRuleInterface, customConfig model.Config, proxyArr map[string][]model.Proxy) model.Rule {
	return mergeRuleInterface.MergeRule(customConfig, proxyArr)
}
