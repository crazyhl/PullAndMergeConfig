package model

import "gopkg.in/yaml.v2"

// dns 配置
type Proxy struct {
	Name           string          `yaml:"name"`
	Type           string          `yaml:"type"`
	Server         string          `yaml:"server"`
	Port           int             `yaml:"port"`
	Cipher         string          `yaml:"cipher,omitempty"`
	Username       string          `yaml:"username,omitempty"`
	Password       string          `yaml:"password,omitempty"`
	Udp            bool            `yaml:"udp,omitempty"`
	Plugin         string          `yaml:"plugin,omitempty"`
	PluginOpts     SSPluginOpts    `yaml:"plugin-opts,omitempty"`
	Uuid           string          `yaml:"uuid,omitempty"`
	AlterId        int             `yaml:"alterId"`
	Tls            bool            `yaml:"tls,omitempty"`
	SkipCertVerify bool            `yaml:"skip-cert-verify,omitempty"`
	Network        string          `yaml:"network,omitempty"`
	WsPath         string          `yaml:"ws-path,omitempty"`
	WsHeaders      yaml.MapSlice   `yaml:"ws-headers,omitempty"`
	Psk            string          `yaml:"psk,omitempty"`
	ObfsOpts       SnellPluginOpts `yaml:"obfs-opts,omitempty"`
}
