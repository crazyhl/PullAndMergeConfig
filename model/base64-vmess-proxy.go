package model

// dns 配置
type Base64VmessProxy struct {
	Name    string `json:"ps"`
	Server  string `json:"add"`
	Port    int    `json:"port"`
	Uuid    string `json:"id,omitempty"`
	AlterId string `json:"aid,omitempty"`
	Tls     string `json:"tls,omitempty"`
	Network string `json:"net,omitempty"`
	WsPath  string `json:"path,omitempty"`
	Host    string `json:"host,omitempty"`
}
