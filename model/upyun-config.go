package model

// dns 配置
type UpYunConfig struct {
	Bucket     string `yaml:"bucket,omitempty"`
	Operator   string `yaml:"operator,omitempty"`
	Password   string `yaml:"password,omitempty"`
	PathPrefix string `yaml:"pathPrefix,omitempty"`
}
