package handlers

import (
	"crypto/md5"
	"fmt"
	"testing"
)

var (
	testDownloadUrl = "https://github.com/docker/machine/releases/download/v0.5.1/docker-machine_linux-amd64.zip"
	checksumMD5     = "4e765b5abac3cb58280514c723536133"
)

func TestDownload(t *testing.T) {
	h := md5.New()
	err := download_driver(testDownloadUrl, h)
	if err != nil {
		t.Fatal("error downloading driver", err)
	}

	obtainedSum := h.Sum(nil)
	if fmt.Sprintf("%x", obtainedSum) != checksumMD5 {
		t.Errorf("Download does not work. Expected=[%s], obtained=[%x]", checksumMD5, obtainedSum)
	}
}
