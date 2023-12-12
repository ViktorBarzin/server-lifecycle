package main

import (
	"flag"
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
	flag.StringVar(&statusFile, "status", "/tmp/server-lifecycle-status.json", "Status file")
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
	// currentState, err := readState(statusFile)
	_, err = readState(statusFile)
	if err != nil {
		return errors.Wrap(err, "error reading current state")
	}

	// Fetch current state and save to disk
	voltage, err := idracClient.AmperageReading()
	if err != nil {
		return errors.Wrap(err, "failed to fetch amperage reading")
	}
	isPoweredOn, err := idracClient.IsPoweredOn()
	if err != nil {
		return errors.Wrap(err, "failed to fetch voltage reading")
	}
	defer updateStateFile(statusFile, ServerState{On: isPoweredOn, Voltage: voltage, LastUpdate: time.Now()})

	if isPoweredOn && voltage > NO_VOLTAGE_THRESHOLD {
		// server is on and there is power - leave
		glog.Info("server is on and there is power")
		return nil
	}
	if isPoweredOn && voltage < NO_VOLTAGE_THRESHOLD {
		// start timer and wait for voltage to come back..
		// perhaps wait until UPS is fully charged? (some hardcoded time)
		glog.Warningf("low voltage detected - %f! Waiting some time before turning off server")
	}
	if !isPoweredOn && voltage < NO_VOLTAGE_THRESHOLD {
		// power off but still no power, so sleep
		glog.Info("server is off but there is still no power so not turning on")
		return nil
	}
	if !isPoweredOn && voltage > NO_VOLTAGE_THRESHOLD {
		// turn on, but perhaps check that UPS is fully charged
		glog.Info("voltage restored! turning on server")
	}
	// if on - update status file and exit
	// if off - check power
	//    if no power - sleep
	//    else - turn on (or maybe sleep to confirm power was on for long enough?)

	// Create an LRU cache with a capacity of 3

	return nil
}

/*
Check status and return an up-to date status
*/
// func refreshStatus(status Status) Status {

// }
