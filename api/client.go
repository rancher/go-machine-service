package api

import (
	"encoding/json"
	"fmt"
	"github.com/rancherio/go-machine-service/utils"
	"io/ioutil"
	"net/http"
)

type Client interface {
	GetPhysicalHost(id string) (*PhysicalHost, error)
}

type clientImpl struct {
	baseUrl       string
	collectionUrl string
	resourceUrl   string
}

func (c *clientImpl) GetPhysicalHost(id string) (*PhysicalHost, error) {
	url := fmt.Sprintf(c.resourceUrl, "physicalhosts", id)
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	var physHost PhysicalHost
	err = json.Unmarshal(body, &physHost)
	if err != nil {
		return nil, err
	}

	return &physHost, nil
}

func NewRestClient() Client {
	base := utils.GetRancherUrl(false) + "/v1"
	collectionUrl := base + "/%v"
	resourceUrl := collectionUrl + "/%v"
	return &clientImpl{
		baseUrl:       base,
		collectionUrl: collectionUrl,
		resourceUrl:   resourceUrl,
	}
}
