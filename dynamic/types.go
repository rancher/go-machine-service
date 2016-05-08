package dynamic

type ResourceFieldConfig struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	Nullable    bool   `json:"nullable,omitempty"`
	Required    bool   `json:"required,omitempty"`
	MinLength   int    `json:"minLength,omitempty"`
	Create      bool   `json:"create,omitempty"`
	Update      bool   `json:"update,omitempty"`
}

type DocumentationFieldConfig struct {
	Description string `json:"description,omitempty"`
}

type DocumentationFields struct {
	ID             string                    `json:"id,omitempty"`
	ResourceFields DocumentationFieldConfigs `json:"resourceFields,omitempty"`
}

type ResourceData struct {
	Blacklist        []string
	Drivers          []string
	ResourceMap      map[string]ResourceFields
	DocumentationMap map[string][]DocumentationFields
}

type ResourceFieldConfigs map[string]ResourceFieldConfig

type ResourceFields map[string]ResourceFieldConfigs

type DocumentationFieldConfigs map[string]DocumentationFieldConfig
