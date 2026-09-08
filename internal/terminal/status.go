package terminal

type TerminalStatus struct {
	Backend string `json:"backend"`
	Ready   bool   `json:"ready"`
	Message string `json:"message"`
}
