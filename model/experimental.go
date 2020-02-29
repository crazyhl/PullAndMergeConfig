package model

import "gopkg.in/yaml.v2"

// 实验性配置
type Experimental struct {
	IgnoreResolveFail bool          `yaml:"ignore-resolve-fail,omitempty"`
	Authentication    []string      `yaml:"authentication,omitempty"`
	Hosts             yaml.MapSlice `yaml:"hosts,omitempty"`
}
