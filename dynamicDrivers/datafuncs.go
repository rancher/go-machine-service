package dynamicDrivers

import (
	"os"
	"path/filepath"
)

const outDir = "generated"

func writeToFile(data []byte, filename string) error {
	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		makeErr := os.Mkdir(outDir, 0666)
		if makeErr != nil {
			return makeErr
		}
	}
	file, err := os.Create(filepath.Join(outDir, filename))
	defer file.Close()
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	return err
}
