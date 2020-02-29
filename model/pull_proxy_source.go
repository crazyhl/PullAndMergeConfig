package model

import "errors"

// 拉取代理源
type PullProxySource struct {
	Name string
	Url  string
}

type PullProxySources []PullProxySource

func (proxies PullProxySources) HasItem(name string) (interface{}, error) {
	for _, proxy := range proxies {
		if proxy.Name == name {
			return proxy, nil
		}
	}
	return nil, errors.New("没有找到匹配的基础规则源")
}
