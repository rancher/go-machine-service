package client

const (
	DYNAMIC_SCHEMA_ROLE_TYPE = "dynamicSchemaRole"
)

type DynamicSchemaRole struct {
	Resource

	DynamicSchemaId string `json:"dynamicSchemaId,omitempty" yaml:"dynamic_schema_id,omitempty"`

	Role string `json:"role,omitempty" yaml:"role,omitempty"`
}

type DynamicSchemaRoleCollection struct {
	Collection
	Data []DynamicSchemaRole `json:"data,omitempty"`
}

type DynamicSchemaRoleClient struct {
	rancherClient *RancherClient
}

type DynamicSchemaRoleOperations interface {
	List(opts *ListOpts) (*DynamicSchemaRoleCollection, error)
	Create(opts *DynamicSchemaRole) (*DynamicSchemaRole, error)
	Update(existing *DynamicSchemaRole, updates interface{}) (*DynamicSchemaRole, error)
	ById(id string) (*DynamicSchemaRole, error)
	Delete(container *DynamicSchemaRole) error
}

func newDynamicSchemaRoleClient(rancherClient *RancherClient) *DynamicSchemaRoleClient {
	return &DynamicSchemaRoleClient{
		rancherClient: rancherClient,
	}
}

func (c *DynamicSchemaRoleClient) Create(container *DynamicSchemaRole) (*DynamicSchemaRole, error) {
	resp := &DynamicSchemaRole{}
	err := c.rancherClient.doCreate(DYNAMIC_SCHEMA_ROLE_TYPE, container, resp)
	return resp, err
}

func (c *DynamicSchemaRoleClient) Update(existing *DynamicSchemaRole, updates interface{}) (*DynamicSchemaRole, error) {
	resp := &DynamicSchemaRole{}
	err := c.rancherClient.doUpdate(DYNAMIC_SCHEMA_ROLE_TYPE, &existing.Resource, updates, resp)
	return resp, err
}

func (c *DynamicSchemaRoleClient) List(opts *ListOpts) (*DynamicSchemaRoleCollection, error) {
	resp := &DynamicSchemaRoleCollection{}
	err := c.rancherClient.doList(DYNAMIC_SCHEMA_ROLE_TYPE, opts, resp)
	return resp, err
}

func (c *DynamicSchemaRoleClient) ById(id string) (*DynamicSchemaRole, error) {
	resp := &DynamicSchemaRole{}
	err := c.rancherClient.doById(DYNAMIC_SCHEMA_ROLE_TYPE, id, resp)
	if apiError, ok := err.(*ApiError); ok {
		if apiError.StatusCode == 404 {
			return nil, nil
		}
	}
	return resp, err
}

func (c *DynamicSchemaRoleClient) Delete(container *DynamicSchemaRole) error {
	return c.rancherClient.doResourceDelete(DYNAMIC_SCHEMA_ROLE_TYPE, &container.Resource)
}
