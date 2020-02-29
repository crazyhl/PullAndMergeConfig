package model

type Config struct {
	BaseRule BaseRules `yaml:"base-rule"`
	PullProxySource PullProxySources `yaml:"pull-proxy-source"`
	Port int `yaml:"port"`
	SockPort int `yaml:"socks-port"`
	AllowLan bool `yaml:"allow-lan"`
	Mode string `yaml:"mode"`
	LogLevel string `yaml:"log-level"`
	ExternalController string `yaml:"external-controller"`
}
