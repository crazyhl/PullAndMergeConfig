package model

type Rule struct {
	Port               int            `yaml:"port,omitempty"`
	SocksPort          int            `yaml:"socks-port,omitempty"`
	RedirPort          int            `yaml:"redir-port,omitempty"`
	AllowLan           bool           `yaml:"allow-lan,omitempty"`
	BindAddress        string         `yaml:"bind-address,omitempty"`
	Mode               string         `yaml:"mode,omitempty"`
	LogLevel           string         `yaml:"log-level,omitempty"`
	ExternalController string         `yaml:"external-controller,omitempty"`
	ExternalUi         string         `yaml:"external-ui,omitempty"`
	Secret             string         `yaml:"secret,omitempty"`
	Experimental       Experimental   `yaml:"experimental,omitempty"`
	Dns                Dns            `yaml:"dns,omitempty"`
	FallbackFilter     FallbackFilter `yaml:"fallback-filter,omitempty"`     // 这个会最终合并到dns 里面去 ，外面也保留一份
	CfwBypass          []string       `yaml:"cfw-bypass,omitempty"`          // 仅在windows下有效
	CfwLatencyTimeout  int            `yaml:"cfw-latency-timeout,omitempty"` // 仅在windows下有效
	Proxy              []Proxy        `yaml:"Proxy,omitempty"`               // 仅在windows下有效
	ProxyGroup         []ProxyGroup   `yaml:"Proxy Group,omitempty"`         // 仅在windows下有效
	Rule               []string       `yaml:"Rule,omitempty"`                // 自定义规则列表
}
