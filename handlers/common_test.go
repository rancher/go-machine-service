package handlers

import (
	v3 "github.com/rancher/go-rancher/v3"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_populateFields(t *testing.T) {
	assert := require.New(t)

	m := &v3.Host{
		Driver: "digitalocean",
		DigitaloceanConfig: &v3.DigitaloceanConfig{
			Region: "sfo2",
			Size:   "1gb",
		},
	}

	populateFields(m)
	dc := m.Data["fields"].(map[string]interface{})["digitaloceanConfig"].(map[string]interface{})
	assert.Equal(m.DigitaloceanConfig.Region, dc["region"])
	assert.Equal(m.DigitaloceanConfig.Size, dc["size"])
}
