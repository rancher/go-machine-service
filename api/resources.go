package api

type PhysicalHost struct {
	Id         string
	ExternalId string
	Type       string
	Kind       string
	Links      map[string]string

	// boot2docker
	Memory         string
	DiskSize       string
	Boot2dockerUrl string

	// digitalocean
	Image       string
	Region      string
	Size        string
	AccessToken string

	// google
	MachineType string
	Project     string
	Username    string
	Zone        string

	// amazonec2
	AccessKey    string
	Ami          string
	InstanceType string
	// Region    string dupe
	RootSize     string
	SecretKey    string
	SessionToken string
	SubnetId     string
	VpcId        string
	// Zone      string dupe
}

/*
{
  "id": "1ph1",
  "type": "dockerMachine",
  "links": {
    "self": "http://localhost:8080/v1/dockermachines/1ph1",
    "account": "http://localhost:8080/v1/dockermachines/1ph1/account",
    "hosts": "http://localhost:8080/v1/dockermachines/1ph1/hosts"
  },
  "actions": {
    "activate": "http://localhost:8080/v1/dockermachines/1ph1/?action=activate",
    "remove": "http://localhost:8080/v1/dockermachines/1ph1/?action=remove",
    "deactivate": "http://localhost:8080/v1/dockermachines/1ph1/?action=deactivate"
  },
  "accountId": "1a1",
  "agentId": null,
  "created": "2015-01-20T03:38:12Z",
  "createdTS": 1421725092000,
  "data": {
    "fields": {
      "digitaloceanImage": "digitaloceanImage4",
      "virtualboxDiskSize": "virtualboxDiskSize2",
      "virtualboxMemory": "virtualboxMemory1",
      "virtualboxBoot2dockerUrl": "virtualboxBoot2dockerUrl3",
      "driver": "virtualbox"
    }
  },
  "description": null,
  "externalId": "c49c7ef5-d487-4250-8621-479b2e05bf89",
  "kind": "dockerMachine",
  "name": "test1",
  "removeTime": null,
  "removed": null,
  "state": "activating",
  "transitioning": "yes",
  "transitioningMessage": "In Progress",
  "transitioningProgress": null,
  "uuid": "28cf372e-4630-49d2-8b6c-5a934542c45b",
  "driver": "virtualbox",
  "virtualboxMemory": "virtualboxMemory1",
  "virtualboxDiskSize": "virtualboxDiskSize2",
  "virtualboxBoot2dockerUrl": "virtualboxBoot2dockerUrl3",
  "digitaloceanImage": "digitaloceanImage4",
  "digitaloceanRegion": null,
  "digitaloceanSize": null,
  "digitaloceanAccessToken": null
}
*/
