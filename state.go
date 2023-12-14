// Manage state file
package main

import (
	"time"

	"github.com/pkg/errors"
)

type ServerState struct {
	On             bool      `json:"on"`
	HasPowerSupply bool      `json:"hasPowerSupply"`
	LastUpdate     time.Time `json:"lastUpdate"`
}

func refreshState(idracClient IDRACClient) (ServerState, error) {
	// return ServerState{On: true, Voltage: 0, LastUpdate: time.Now()}, nil // mock
	hasPowerSupply, err := idracClient.HasPowerSupply()
	if err != nil {
		return ServerState{}, errors.Wrap(err, "failed to fetch amperage reading")
	}
	isPoweredOn, err := idracClient.IsPoweredOn()
	if err != nil {
		return ServerState{}, errors.Wrap(err, "failed to fetch voltage reading")
	}
	return ServerState{On: isPoweredOn, HasPowerSupply: hasPowerSupply, LastUpdate: time.Now()}, nil
}
