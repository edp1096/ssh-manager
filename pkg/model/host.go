package model

type HostList struct {
	Categories []HostCategory `json:"host-categories"`
}

type HostCategory struct {
	Name  string     `json:"name"`
	Hosts []HostInfo `json:"hosts"`
}

type HostInfo struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"-"`
	PrivateKeyText string `json:"private-key-text"`
	UniqueID       string `json:"unique-id"`
}

type HostRequestInfo struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	PrivateKeyText string `json:"private-key-text"`
	UniqueID       string `json:"unique-id"`
}

func GetEmptyHostList() HostList {
	return HostList{
		Categories: []HostCategory{
			{Name: "Default", Hosts: []HostInfo{}},
		},
	}
}
