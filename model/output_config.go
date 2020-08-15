package model

// 输出的配置项
type OutputConfig struct {
	Port               interface{}   `yaml:"port,omitempty"`
	SocksPort          interface{}   `yaml:"socks-port,omitempty"`
	RedirPort          interface{}   `yaml:"redir-port,omitempty"`
	MixedPort          interface{}   `yaml:"mixed-port,omitempty"`
	AllowLan           interface{}   `yaml:"allow-lan,omitempty"`
	BindAddress        interface{}   `yaml:"bind-address,omitempty"`
	Mode               interface{}   `yaml:"mode,omitempty"`
	LogLevel           interface{}   `yaml:"log-level,omitempty"`
	Ipv6               interface{}   `yaml:"ipv6,omitempty"`
	ExternalController interface{}   `yaml:"external-controller,omitempty"`
	ExternalUi         interface{}   `yaml:"external-ui,omitempty"`
	InterfaceName      interface{}   `yaml:"interface-name,omitempty"`
	Hosts              []interface{} `yaml:"hosts,omitempty"`
	Dns                []interface{} `yaml:"dns,omitempty"`
	Proxies            []interface{} `yaml:"proxies,omitempty"`
	ProxyGroups        []interface{} `yaml:"proxy-groups,omitempty"`
	ProxyProviders     []interface{} `yaml:"proxy-providers,omitempty"`
	Rules              []interface{} `yaml:"rules,omitempty"`
}
