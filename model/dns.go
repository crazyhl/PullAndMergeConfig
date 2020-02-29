package model

// dns 配置
type Dns struct {
	EnableDns      bool           `yaml:"enable,omitempty"`
	Ipv6           bool           `yaml:"ipv6,omitempty"`
	EnhancedMode   string         `yaml:"enhanced-mode,omitempty"`
	FakeIpRange    string         `yaml:"fake-ip-range,omitempty"`
	FakeIpFilter   []string       `yaml:"fake-ip-filter,omitempty"`
	NameServer     []string       `yaml:"nameserver,omitempty"`
	Fallback       []string       `yaml:"fallback,omitempty"`
	FallbackFilter FallbackFilter `yaml:"fallback-filter,omitempty"`
}
