package helpers

type ResourceFieldConfig struct {
	Type     string `json:"type,omitempty"`
	Nullable bool   `json:"nullable,omitempty"`
	Required bool   `json:"required,omitempty"`
}

type ResourceData struct {
	Blacklist   []string
	Drivers     []string
	ResourceMap map[string]ResourceFields
}

type ResourceFieldConfigs map[string]ResourceFieldConfig

type ResourceFields map[string]ResourceFieldConfigs
