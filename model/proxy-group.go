package model

// dns 配置
type ProxyGroup struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies"`
	Url      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
}
