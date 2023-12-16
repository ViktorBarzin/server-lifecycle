package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

type AuthContext struct {
	Host     string
	Username string
	Password string
}

func main() {
	if err := run(); err != nil {
		glog.Fatal("run failed: " + err.Error())
	}
}

func run() error {
	var statusFile string
	var runLockFile string
	var idracHost string
	var idracUsername string
	var idracPassword string
	var turnOffThreshdoldMinutes int
	var pollIntervalSeconds int

	flag.StringVar(&idracHost, "host", "idrac", "Idrac host")
	flag.StringVar(&statusFile, "status-file", "/tmp/server-lifecycle-status.json", "Status file")
	flag.StringVar(&idracUsername, "user", "root", "Idrac user")
	flag.StringVar(&idracPassword, "password", "calvin", "Idrac password")
	flag.StringVar(&runLockFile, "run-lock", "/tmp/server-lifecycle.lock", "Run lock to ensure at most 1 instance of the script is running")
	flag.IntVar(&turnOffThreshdoldMinutes, "turnoff-threshold", 15, "Minutes to wait before turning off server. Time since last update is taken into account.")
	flag.IntVar(&pollIntervalSeconds, "poll-interval-seconds", 30, "Seconds to wait between polling for power status when power goes away.")
	flag.Parse()

	if _, err := os.Stat(runLockFile); err == nil {
		glog.Warningf("could not obtain lock file %s. perhaps another instance of this script is running. please wait until it completes or manually remove the lock file", runLockFile)
		return nil
	} else {
		glog.Info("no other running instance found, starting lifecycle checks")
	}
	err := initStateFile(statusFile)
	if err != nil {
		return errors.Wrap(err, "failed to init state file")
	}
	// fetch laste updated time
	savedState, err := readState(statusFile)
	if err != nil {
		return errors.Wrap(err, "error reading current state")
	}
	lastUpdate := savedState.LastUpdate

	_, err = os.Create(runLockFile)
	if err != nil {
		return errors.Wrap(err, "failed to create lock file")
	}
	defer os.Remove(runLockFile)

	defer glog.Flush()
	authContext := AuthContext{Host: idracHost, Username: idracUsername, Password: idracPassword}
	idracClient := IDRACClient{authContext: authContext, cache: NewLRUCache(20)}

	// Fetch current state and save to disk
	currentState, err := refreshState(idracClient)
	currentState.LastUpdate = lastUpdate // use last update
	if err != nil {
		return errors.Wrap(err, "failed to fetch current state")
	}

	if currentState.On && currentState.HasPowerSupply {
		// server is on and there is power - leave
		glog.Info("server is on and there is power")
		currentState.LastUpdate = time.Now()
		updateStateFile(statusFile, currentState)
		return nil
	}
	if currentState.On && !currentState.HasPowerSupply {
		// start timer and wait for voltage to come back..
		// perhaps wait until UPS is fully charged? (some hardcoded time)
		turnOffThreshold := time.Minute*time.Duration(turnOffThreshdoldMinutes) - time.Now().Sub(currentState.LastUpdate)
		pollInterval := time.Second * time.Duration(pollIntervalSeconds)
		err = handlePowerOnNoVoltage(currentState, idracClient, turnOffThreshold, pollInterval)
		if err != nil {
			return errors.Wrap(err, "error handling no power while server is on")
		}
		refreshAndSaveState(idracClient, statusFile)
		return nil
	}
	if !currentState.On && !currentState.HasPowerSupply {
		// power off but still no power, so sleep
		glog.Info("server is off but there is still no power so not turning on")
		currentState.LastUpdate = time.Now()
		updateStateFile(statusFile, currentState)
		return nil
	}
	if !currentState.On && currentState.HasPowerSupply {
		// turn on, but perhaps check that UPS is fully charged
		glog.Info("voltage restored! turning on server")
		handlePowerOffWithVoltage(idracClient)
		refreshAndSaveState(idracClient, statusFile)
		return nil
	}
	return fmt.Errorf("unexpected combination of server state: %t, has power supply: %t", currentState.On, currentState.HasPowerSupply)
}

/* Handle case where power was lost while server is on. */
func handlePowerOnNoVoltage(currentState ServerState, idracClient IDRACClient, turnOffThreshold time.Duration, pollInterval time.Duration) error {
	glog.Warningf("no power detected! Waiting %f minutes before turning off server", turnOffThreshold.Minutes())
	turnOffChannel := time.After(turnOffThreshold)

	for {
		glog.Infof("rechecking system state in %f seconds...", pollInterval.Seconds())
		select {
		case <-time.After(pollInterval):
			glog.Info("rechecking system state")
			// query again
			// if voltage restored, break
			currentState, err := refreshState(idracClient)
			if err != nil {
				glog.Error("failed to refetch current state: " + err.Error())
				continue
			}
			if currentState.HasPowerSupply {
				glog.Info("power is restored")
				return nil
			} else {
				glog.Info("power is still off")
			}
		case <-turnOffChannel:
			// turn off server
			glog.Warningf("timeout of %f seconds elapsed. powering off server", turnOffThreshold.Seconds())
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

// Useful to refresh state on exit
func refreshAndSaveState(idracClient IDRACClient, statusFile string) {
	glog.Info("refreshing state after script has run to saved new state")
	sleep := time.Duration(15 * time.Second)
	glog.Infof("sleeping %f seconds to ensure all changes have been propagated", sleep.Seconds())
	time.Sleep(sleep)
	// refresh state at exit and write state after script has run
	currentState, err := refreshState(idracClient)
	if err == nil {
		updateStateFile(statusFile, currentState)
	}
}
