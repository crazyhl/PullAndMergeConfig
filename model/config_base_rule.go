package model

import "errors"

// 基础规则源
type ConfigBaseRule struct {
	Name string
	Url  string
}

type ConfigBaseRules []ConfigBaseRule

func (rules ConfigBaseRules) HasItem(name string) (interface{}, error) {
	for _, rule := range rules {
		if rule.Name == name {
			return rule, nil
		}
	}
	return nil, errors.New("没有找到匹配的基础规则源")
}
