// Manage state file
package main

import (
	"encoding/json"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

type ServerState struct {
	On         bool      `json:"on"`
	Voltage    float64   `json:"voltage"`
	LastUpdate time.Time `json:"lastUpdate"`
}

func initStateFile(path string) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		glog.Infof("state file %s already exists, not overwriting...", path)
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		glog.Warningln("Error creating file: " + err.Error())
		return err
	}
	defer file.Close()

	// Create a JSON encoder
	encoder := json.NewEncoder(file)

	// Encode the struct and write to the file
	err = encoder.Encode(ServerState{})
	if err != nil {
		return err
	}
	defaultStatus := ServerState{LastUpdate: time.Now()}
	glog.Infof("Writing status: %+v", defaultStatus)
	return nil
}

func readState(path string) (ServerState, error) {
	file, err := os.Open(path)
	if err != nil {
		return ServerState{}, err
	}
	decoder := json.NewDecoder(file)
	var status ServerState
	err = decoder.Decode(&status)
	return status, err
}

func updateStateFile(path string, state ServerState) error {
	encoded, err := json.Marshal(state)
	glog.Infof("serializing state %+v to file %s", state, path)
	err = os.WriteFile(path, encoded, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write serialized state to file")
	}
	return nil
}
