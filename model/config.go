package model

type Config struct {
	ConfigBaseRule     ConfigBaseRules  `yaml:"base-rule"`
	PullProxySource    PullProxySources `yaml:"pull-proxy-source"`
	Port               int              `yaml:"port,omitempty"`
	SocksPort          int              `yaml:"socks-port,omitempty"`
	AllowLan           bool             `yaml:"allow-lan"`
	Mode               string           `yaml:"mode,omitempty"`
	LogLevel           string           `yaml:"log-level,omitempty"`
	ExternalController string           `yaml:"external-controller,omitempty"`
	ExternalUi         string           `yaml:"external-ui,omitempty"`
	Secret             string           `yaml:"secret,omitempty"`
	Experimental       Experimental     `yaml:"experimental,omitempty"`
	Dns                Dns              `yaml:"dns,omitempty"`
	FallbackFilter     FallbackFilter   `yaml:"fallback-filter,omitempty"`     // 这个会最终合并到dns 里面去
	CfwBypass          []string         `yaml:"cfw-bypass,omitempty"`          // 仅在windows下有效
	CfwLatencyTimeout  int              `yaml:"cfw-latency-timeout,omitempty"` // 仅在windows下有效
	Proxy              []Proxy          `yaml:"Proxy,omitempty"`               // 这里还可以放一些自定义的代理信息上去哟
	ProxyGroup         []ProxyGroup     `yaml:"Proxy Group,omitempty"`         // 最终会合并到配置文件中，并且这里面的 所有，都会合并到 proxy 组中去
	Rule               []string         `yaml:"Rule,omitempty"`                // 自定义规则列表 最终会合并到配置文件中
}
