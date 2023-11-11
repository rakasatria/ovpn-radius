package main

type Config struct {
	LogFile    string           `json:"LogFile"`
	ServerInfo ConfigServerInfo `json:"ServerInfo"`
	Radius     ConfigRadius     `json:"Radius"`
}

type ConfigServerInfo struct {
	Identifier  string `json:"Identifier"`
	IpAddress   string `json:"IpAddress"`
	PortType    string `json:"PortType"`
	ServiceType string `json:"ServiceType"`
}

type ConfigRadius struct {
	AuthenticationOnly	bool		 `json:"AuthenticationOnly"`
	Authentication 		ConfigServer `json:"Authentication"`
	Accounting     		ConfigServer `json:"Accounting"`
}

type ConfigServer struct {
	Server string `json:"Server"`
	Secret string `json:"Secret"`
}
