package client

const (
	VMWAREFUSION_CONFIG_TYPE = "vmwarefusionConfig"
)

type VmwarefusionConfig struct {
	Resource

	ActiveTimeout string `json:"activeTimeout,omitempty" yaml:"active_timeout,omitempty"`

	AuthUrl string `json:"authUrl,omitempty" yaml:"auth_url,omitempty"`

	AvailabilityZone string `json:"availabilityZone,omitempty" yaml:"availability_zone,omitempty"`

	DomainId string `json:"domainId,omitempty" yaml:"domain_id,omitempty"`

	DomainName string `json:"domainName,omitempty" yaml:"domain_name,omitempty"`

	EndpointType string `json:"endpointType,omitempty" yaml:"endpoint_type,omitempty"`

	FlavorId string `json:"flavorId,omitempty" yaml:"flavor_id,omitempty"`

	FlavorName string `json:"flavorName,omitempty" yaml:"flavor_name,omitempty"`

	FloatingipPool string `json:"floatingipPool,omitempty" yaml:"floatingip_pool,omitempty"`

	ImageId string `json:"imageId,omitempty" yaml:"image_id,omitempty"`

	ImageName string `json:"imageName,omitempty" yaml:"image_name,omitempty"`

	Insecure bool `json:"insecure,omitempty" yaml:"insecure,omitempty"`

	IpVersion string `json:"ipVersion,omitempty" yaml:"ip_version,omitempty"`

	NetId string `json:"netId,omitempty" yaml:"net_id,omitempty"`

	NetName string `json:"netName,omitempty" yaml:"net_name,omitempty"`

	NovaNetwork bool `json:"novaNetwork,omitempty" yaml:"nova_network,omitempty"`

	Password string `json:"password,omitempty" yaml:"password,omitempty"`

	Region string `json:"region,omitempty" yaml:"region,omitempty"`

	SecGroups string `json:"secGroups,omitempty" yaml:"sec_groups,omitempty"`

	SshPort string `json:"sshPort,omitempty" yaml:"ssh_port,omitempty"`

	SshUser string `json:"sshUser,omitempty" yaml:"ssh_user,omitempty"`

	TenantId string `json:"tenantId,omitempty" yaml:"tenant_id,omitempty"`

	TenantName string `json:"tenantName,omitempty" yaml:"tenant_name,omitempty"`

	Username string `json:"username,omitempty" yaml:"username,omitempty"`
}

type VmwarefusionConfigCollection struct {
	Collection
	Data []VmwarefusionConfig `json:"data,omitempty"`
}

type VmwarefusionConfigClient struct {
	rancherClient *RancherClient
}

type VmwarefusionConfigOperations interface {
	List(opts *ListOpts) (*VmwarefusionConfigCollection, error)
	Create(opts *VmwarefusionConfig) (*VmwarefusionConfig, error)
	Update(existing *VmwarefusionConfig, updates interface{}) (*VmwarefusionConfig, error)
	ById(id string) (*VmwarefusionConfig, error)
	Delete(container *VmwarefusionConfig) error
}

func newVmwarefusionConfigClient(rancherClient *RancherClient) *VmwarefusionConfigClient {
	return &VmwarefusionConfigClient{
		rancherClient: rancherClient,
	}
}

func (c *VmwarefusionConfigClient) Create(container *VmwarefusionConfig) (*VmwarefusionConfig, error) {
	resp := &VmwarefusionConfig{}
	err := c.rancherClient.doCreate(VMWAREFUSION_CONFIG_TYPE, container, resp)
	return resp, err
}

func (c *VmwarefusionConfigClient) Update(existing *VmwarefusionConfig, updates interface{}) (*VmwarefusionConfig, error) {
	resp := &VmwarefusionConfig{}
	err := c.rancherClient.doUpdate(VMWAREFUSION_CONFIG_TYPE, &existing.Resource, updates, resp)
	return resp, err
}

func (c *VmwarefusionConfigClient) List(opts *ListOpts) (*VmwarefusionConfigCollection, error) {
	resp := &VmwarefusionConfigCollection{}
	err := c.rancherClient.doList(VMWAREFUSION_CONFIG_TYPE, opts, resp)
	return resp, err
}

func (c *VmwarefusionConfigClient) ById(id string) (*VmwarefusionConfig, error) {
	resp := &VmwarefusionConfig{}
	err := c.rancherClient.doById(VMWAREFUSION_CONFIG_TYPE, id, resp)
	if apiError, ok := err.(*ApiError); ok {
		if apiError.StatusCode == 404 {
			return nil, nil
		}
	}
	return resp, err
}

func (c *VmwarefusionConfigClient) Delete(container *VmwarefusionConfig) error {
	return c.rancherClient.doResourceDelete(VMWAREFUSION_CONFIG_TYPE, &container.Resource)
}
