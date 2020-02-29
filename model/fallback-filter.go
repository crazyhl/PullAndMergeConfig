package model

// fallback-filter 配置
type FallbackFilter struct {
	GeoIp  bool     `yaml:"geoip,omitempty"`
	IpcIdr []string `yaml:"ipcidr,omitempty"`
}
