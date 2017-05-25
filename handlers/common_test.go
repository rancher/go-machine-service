package handlers

import (
	"github.com/rancher/go-rancher/v2"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_populateFields(t *testing.T) {
	assert := require.New(t)

	m := &client.Machine{
		Driver: "digitalocean",
		DigitaloceanConfig: &client.DigitaloceanConfig{
			Region: "sfo2",
			Size:   "1gb",
		},
	}

	populateFields(m)
	dc := m.Data["fields"].(map[string]interface{})["digitaloceanConfig"].(map[string]interface{})
	assert.Equal(m.DigitaloceanConfig.Region, dc["region"])
	assert.Equal(m.DigitaloceanConfig.Size, dc["size"])
}
