package models

// Session represents a VCD-compliant session object
type Session struct {
	ID                        string      `json:"id"`
	Site                      EntityRef   `json:"site"`
	User                      EntityRef   `json:"user"`
	Org                       EntityRef   `json:"org"`
	OperatingOrg              EntityRef   `json:"operatingOrg"`
	Location                  string      `json:"location"`
	Roles                     []string    `json:"roles"`
	RoleRefs                  []EntityRef `json:"roleRefs"`
	SessionIdleTimeoutMinutes int         `json:"sessionIdleTimeoutMinutes"`
}
