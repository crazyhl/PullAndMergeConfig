package model

// 实验性配置
type Experimental struct {
	IgnoreResolveFail bool `yaml:"ignore-resolve-fail"`
	Authentication []string `yaml:"authentication"`
	Hosts map[string]string `yaml:"hosts"`
}