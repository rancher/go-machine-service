package client

const (
	VMWAREVCLOUDAIR_CONFIG_TYPE = "vmwarevcloudairConfig"
)

type VmwarevcloudairConfig struct {
	Resource

	ApiEndpoint string `json:"apiEndpoint,omitempty" yaml:"api_endpoint,omitempty"`

	ApiKey string `json:"apiKey,omitempty" yaml:"api_key,omitempty"`

	Cpu string `json:"cpu,omitempty" yaml:"cpu,omitempty"`

	DiskSize string `json:"diskSize,omitempty" yaml:"disk_size,omitempty"`

	Domain string `json:"domain,omitempty" yaml:"domain,omitempty"`

	Hostname string `json:"hostname,omitempty" yaml:"hostname,omitempty"`

	HourlyBilling bool `json:"hourlyBilling,omitempty" yaml:"hourly_billing,omitempty"`

	Image string `json:"image,omitempty" yaml:"image,omitempty"`

	LocalDisk bool `json:"localDisk,omitempty" yaml:"local_disk,omitempty"`

	Memory string `json:"memory,omitempty" yaml:"memory,omitempty"`

	PrivateNetOnly bool `json:"privateNetOnly,omitempty" yaml:"private_net_only,omitempty"`

	PrivateVlanId string `json:"privateVlanId,omitempty" yaml:"private_vlan_id,omitempty"`

	PublicVlanId string `json:"publicVlanId,omitempty" yaml:"public_vlan_id,omitempty"`

	Region string `json:"region,omitempty" yaml:"region,omitempty"`

	User string `json:"user,omitempty" yaml:"user,omitempty"`
}

type VmwarevcloudairConfigCollection struct {
	Collection
	Data []VmwarevcloudairConfig `json:"data,omitempty"`
}

type VmwarevcloudairConfigClient struct {
	rancherClient *RancherClient
}

type VmwarevcloudairConfigOperations interface {
	List(opts *ListOpts) (*VmwarevcloudairConfigCollection, error)
	Create(opts *VmwarevcloudairConfig) (*VmwarevcloudairConfig, error)
	Update(existing *VmwarevcloudairConfig, updates interface{}) (*VmwarevcloudairConfig, error)
	ById(id string) (*VmwarevcloudairConfig, error)
	Delete(container *VmwarevcloudairConfig) error
}

func newVmwarevcloudairConfigClient(rancherClient *RancherClient) *VmwarevcloudairConfigClient {
	return &VmwarevcloudairConfigClient{
		rancherClient: rancherClient,
	}
}

func (c *VmwarevcloudairConfigClient) Create(container *VmwarevcloudairConfig) (*VmwarevcloudairConfig, error) {
	resp := &VmwarevcloudairConfig{}
	err := c.rancherClient.doCreate(VMWAREVCLOUDAIR_CONFIG_TYPE, container, resp)
	return resp, err
}

func (c *VmwarevcloudairConfigClient) Update(existing *VmwarevcloudairConfig, updates interface{}) (*VmwarevcloudairConfig, error) {
	resp := &VmwarevcloudairConfig{}
	err := c.rancherClient.doUpdate(VMWAREVCLOUDAIR_CONFIG_TYPE, &existing.Resource, updates, resp)
	return resp, err
}

func (c *VmwarevcloudairConfigClient) List(opts *ListOpts) (*VmwarevcloudairConfigCollection, error) {
	resp := &VmwarevcloudairConfigCollection{}
	err := c.rancherClient.doList(VMWAREVCLOUDAIR_CONFIG_TYPE, opts, resp)
	return resp, err
}

func (c *VmwarevcloudairConfigClient) ById(id string) (*VmwarevcloudairConfig, error) {
	resp := &VmwarevcloudairConfig{}
	err := c.rancherClient.doById(VMWAREVCLOUDAIR_CONFIG_TYPE, id, resp)
	if apiError, ok := err.(*ApiError); ok {
		if apiError.StatusCode == 404 {
			return nil, nil
		}
	}
	return resp, err
}

func (c *VmwarevcloudairConfigClient) Delete(container *VmwarevcloudairConfig) error {
	return c.rancherClient.doResourceDelete(VMWAREVCLOUDAIR_CONFIG_TYPE, &container.Resource)
}
