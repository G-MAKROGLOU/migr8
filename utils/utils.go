package utils

import (
	"encoding/json"
	"os"

	"github.com/fatih/color"
)

// ReadJSON reads a json file and deserializes it into K
func ReadJSON[K interface{}](path string, model K) error {
	config, readErr := os.ReadFile(path)
	if readErr != nil {
		color.Red("[ERR:] => READ FILE => %s", readErr.Error())
		return readErr
	}

	configErr := json.Unmarshal(config, &model)

	if configErr != nil {
		color.Red("[ERR:] => JSON UNMARSHAL => %s", configErr.Error())
		return configErr
	}
	return nil
}
