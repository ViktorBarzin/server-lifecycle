package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

type IDRACClient struct {
	authContext AuthContext
	cache       *LRUCache
}

func (c *IDRACClient) fetch(path string, method string) (map[string]interface{}, error) {
	if body, exists := c.cache.Get(path); exists {
		return body.(map[string]interface{}), nil
	}
	url := fmt.Sprintf("https://%s%s", c.authContext.Host, path)

	// Set your Basic Authentication credentials
	auth := c.authContext.Username + ":" + c.authContext.Password
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	// Create an HTTP client
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Create an HTTP GET request with Basic Authentication
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return map[string]interface{}{}, errors.Wrap(err, "error creating request")
	}

	// Set the Authorization header
	request.Header.Set("Authorization", authHeader)

	// Send the HTTP request
	response, err := client.Do(request)
	if err != nil {
		return map[string]interface{}{}, errors.Wrap(err, "error sending request")
	}
	defer response.Body.Close()

	// Check if the response status code is OK (200)
	if response.StatusCode != http.StatusOK {
		return map[string]interface{}{}, errors.Wrap(err, "request failed with status")
	}

	// Read the response body
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return map[string]interface{}{}, errors.Wrap(err, "error reading response body")
	}

	// Print the response body as a string
	var decodedBody map[string]interface{}
	err = json.Unmarshal(body, &decodedBody)
	if err != nil {
		return map[string]interface{}{}, errors.Wrap(err, "failed json decoding body")
	}
	c.cache.Put(path, decodedBody)
	return decodedBody, nil
}

func (c *IDRACClient) IsPoweredOn() (bool, error) {
	const poweredOnPath = "/redfish/v1/Chassis/System.Embedded.1"
	glog.Info("fetching server power state")
	body, err := c.fetch(poweredOnPath, "GET")
	if err != nil {
		return false, err
	}
	powerState := body["PowerState"]
	return powerState == "On", nil
}

func (c *IDRACClient) AmperageReading() (float64, error) {
	const amperatePath = "/redfish/v1/Chassis/System.Embedded.1/Power/PowerSupplies/PSU.Slot.2"
	glog.Info("fetching voltage state")
	body, err := c.fetch(amperatePath, "GET")
	if err != nil {
		return 0, errors.Wrap(err, "failed to fetch amperage from idrac")
	}
	inputVoltage := body["LineInputVoltage"].(float64)
	return inputVoltage, nil
}
