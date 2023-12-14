package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

type AuthContext struct {
	Host     string
	Username string
	Password string
}

const NO_VOLTAGE_THRESHOLD = 100 // Below this threshold, assume as there is no voltage

func main() {
	if err := run(); err != nil {
		glog.Fatal("run failed: " + err.Error())
	}
}

func run() error {
	var statusFile string
	var idracHost string
	var idracUsername string
	var idracPassword string

	flag.StringVar(&idracHost, "host", "idrac", "Idrac host")
	flag.StringVar(&statusFile, "status-file", "/tmp/server-lifecycle-status.json", "Status file")
	flag.StringVar(&idracUsername, "user", "root", "Idrac user")
	flag.StringVar(&idracPassword, "password", "calvin", "Idrac password")

	flag.Parse()
	defer glog.Flush()
	authContext := AuthContext{Host: idracHost, Username: idracUsername, Password: idracPassword}
	idracClient := IDRACClient{authContext: authContext, cache: NewLRUCache(20)}

	err := initStateFile(statusFile)
	if err != nil {
		return errors.Wrap(err, "failed to init state file")
	}
	savedState, err := readState(statusFile)
	if err != nil {
		return errors.Wrap(err, "error reading current state")
	}

	// Fetch current state and save to disk
	currentState, err := refreshState(idracClient)
	if err != nil {
		return errors.Wrap(err, "failed to fetch current state")
	}
	defer updateStateFile(statusFile, currentState)

	if currentState.On && currentState.Voltage > NO_VOLTAGE_THRESHOLD {
		// server is on and there is power - leave
		glog.Info("server is on and there is power")
		return nil
	}
	if currentState.On && currentState.Voltage < NO_VOLTAGE_THRESHOLD {
		// start timer and wait for voltage to come back..
		// perhaps wait until UPS is fully charged? (some hardcoded time)
		err = handlePowerOnNoVoltage(savedState, idracClient)
		if err != nil {
			return errors.Wrap(err, "error handling no power while server is on")
		}
		return nil
	}
	if !currentState.On && currentState.Voltage < NO_VOLTAGE_THRESHOLD {
		// power off but still no power, so sleep
		glog.Info("server is off but there is still no power so not turning on")
		return nil
	}
	if !currentState.On && currentState.Voltage > NO_VOLTAGE_THRESHOLD {
		// turn on, but perhaps check that UPS is fully charged
		glog.Info("voltage restored! turning on server")
		handlePowerOffWithVoltage(idracClient)
		return nil
	}
	return fmt.Errorf("unexpected combination of server state: %t, voltage: %f", currentState.On, currentState.Voltage)
}

/* Handle case where power was lost while server is on. */
func handlePowerOnNoVoltage(currentState ServerState, idracClient IDRACClient) error {
	turnOffThreshdold := time.Minute*20 - time.Now().Sub(currentState.LastUpdate) // 20 minutes - time since last update
	glog.Warningf("low voltage detected - %f! Waiting %f minutes before turning off server", currentState.Voltage, turnOffThreshdold.Minutes())
	turnOffChannel := time.After(turnOffThreshdold)
	pollInterval := time.Minute * 1

	for {
		glog.Infof("rechecking system state in %f seconds...", pollInterval.Seconds())
		select {
		case <-time.After(pollInterval):
			glog.Info("rechecking system state")
			// query again
			// if voltage restored, break
			voltage, err := idracClient.AmperageReading()
			if err != nil {
				glog.Error("failed to fetch voltage reading: " + err.Error())
				// return errors.Wrap(err, "failed to fetch amperage reading") // should we return ot keep trying?
				continue
			}
			if voltage > NO_VOLTAGE_THRESHOLD {
				glog.Infof("power is restored, current reading: %f", voltage)
				return nil
			} else {
				glog.Infof("voltage %f is still below threshold %d", voltage, NO_VOLTAGE_THRESHOLD)
			}
		case <-turnOffChannel:
			// turn off server
			glog.Warningf("timeout of %f seconds elapsed. powering off server", turnOffThreshdold.Seconds())
			response, err := idracClient.TurnOff()
			if err != nil {
				return errors.Wrap(err, "failed to turn off server")
			}
			glog.Infof("received response from tuning off server: %+v", response)
			return nil
		}
	}
}

func handlePowerOffWithVoltage(idracClient IDRACClient) error {
	// perhaps check UPS battery and or time since power is on
	// this will help avoid turning it on too soon (rare case now)
	response, err := idracClient.TurnOn()
	if err != nil {
		return errors.Wrap(err, "error turning on server")
	}
	glog.Infof("received response form tuning on server: %+v", response)
	return nil
}
