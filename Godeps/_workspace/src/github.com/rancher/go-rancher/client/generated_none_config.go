package client

const (
	NONE_CONFIG_TYPE = "noneConfig"
)

type NoneConfig struct {
	Resource

	Address string `json:"address,omitempty" yaml:"address,omitempty"`

	DiskSize string `json:"diskSize,omitempty" yaml:"disk_size,omitempty"`

	DiskType string `json:"diskType,omitempty" yaml:"disk_type,omitempty"`

	MachineImage string `json:"machineImage,omitempty" yaml:"machine_image,omitempty"`

	MachineType string `json:"machineType,omitempty" yaml:"machine_type,omitempty"`

	Preemptible bool `json:"preemptible,omitempty" yaml:"preemptible,omitempty"`

	Project string `json:"project,omitempty" yaml:"project,omitempty"`

	Scopes string `json:"scopes,omitempty" yaml:"scopes,omitempty"`

	Tags string `json:"tags,omitempty" yaml:"tags,omitempty"`

	UseInternalIp bool `json:"useInternalIp,omitempty" yaml:"use_internal_ip,omitempty"`

	Username string `json:"username,omitempty" yaml:"username,omitempty"`

	Zone string `json:"zone,omitempty" yaml:"zone,omitempty"`
}

type NoneConfigCollection struct {
	Collection
	Data []NoneConfig `json:"data,omitempty"`
}

type NoneConfigClient struct {
	rancherClient *RancherClient
}

type NoneConfigOperations interface {
	List(opts *ListOpts) (*NoneConfigCollection, error)
	Create(opts *NoneConfig) (*NoneConfig, error)
	Update(existing *NoneConfig, updates interface{}) (*NoneConfig, error)
	ById(id string) (*NoneConfig, error)
	Delete(container *NoneConfig) error
}

func newNoneConfigClient(rancherClient *RancherClient) *NoneConfigClient {
	return &NoneConfigClient{
		rancherClient: rancherClient,
	}
}

func (c *NoneConfigClient) Create(container *NoneConfig) (*NoneConfig, error) {
	resp := &NoneConfig{}
	err := c.rancherClient.doCreate(NONE_CONFIG_TYPE, container, resp)
	return resp, err
}

func (c *NoneConfigClient) Update(existing *NoneConfig, updates interface{}) (*NoneConfig, error) {
	resp := &NoneConfig{}
	err := c.rancherClient.doUpdate(NONE_CONFIG_TYPE, &existing.Resource, updates, resp)
	return resp, err
}

func (c *NoneConfigClient) List(opts *ListOpts) (*NoneConfigCollection, error) {
	resp := &NoneConfigCollection{}
	err := c.rancherClient.doList(NONE_CONFIG_TYPE, opts, resp)
	return resp, err
}

func (c *NoneConfigClient) ById(id string) (*NoneConfig, error) {
	resp := &NoneConfig{}
	err := c.rancherClient.doById(NONE_CONFIG_TYPE, id, resp)
	if apiError, ok := err.(*ApiError); ok {
		if apiError.StatusCode == 404 {
			return nil, nil
		}
	}
	return resp, err
}

func (c *NoneConfigClient) Delete(container *NoneConfig) error {
	return c.rancherClient.doResourceDelete(NONE_CONFIG_TYPE, &container.Resource)
}
