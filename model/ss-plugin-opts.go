package model

import "gopkg.in/yaml.v2"

// SSPluginOpts 配置
type SSPluginOpts struct {
	Mode           string        `yaml:"mode,omitempty"`
	Host           string        `yaml:"host,omitempty"`
	Tls            string          `yaml:"tls,omitempty"`
	SkipCertVerify bool          `yaml:"skip-cert-verify,omitempty"`
	Path           string        `yaml:"path,omitempty"`
	Headers        yaml.MapSlice `yaml:"headers,omitempty"`
}
