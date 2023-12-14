package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

type IDRACClient struct {
	authContext AuthContext
	cache       *LRUCache
}

func (c *IDRACClient) doRequest(request *http.Request) (map[string]interface{}, error) {
	// Set your Basic Authentication credentials
	auth := c.authContext.Username + ":" + c.authContext.Password
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	// Create an HTTP client
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
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
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return map[string]interface{}{}, errors.Wrap(err, "error reading response body")
	}

	// Print the response body as a string
	var decodedBody map[string]interface{}
	err = json.Unmarshal(body, &decodedBody)
	if err != nil {
		return map[string]interface{}{}, errors.Wrap(err, "failed json decoding body")
	}
	return decodedBody, nil

}

func (c *IDRACClient) fetch(path string) (map[string]interface{}, error) {
	// if body, exists := c.cache.Get(path); exists {
	// 	return body.(map[string]interface{}), nil
	// }
	url := fmt.Sprintf("https://%s%s", c.authContext.Host, path)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return map[string]interface{}{}, errors.Wrap(err, "error creating request")
	}
	response, err := c.doRequest(request)
	if err != nil {
		return map[string]interface{}{}, errors.Wrapf(err, "failed to fetch path %s", path)
	}
	// c.cache.Put(path, response)
	return response, nil
}

func (c *IDRACClient) post(path string, data []byte) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://%s%s", c.authContext.Host, path)
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return map[string]interface{}{}, errors.Wrap(err, "error creating request")
	}
	response, err := c.doRequest(request)
	if err != nil {
		return map[string]interface{}{}, errors.Wrapf(err, "failed to post to path %s", path)
	}
	return response, nil
}

func (c *IDRACClient) IsPoweredOn() (bool, error) {
	const poweredOnPath = "/redfish/v1/Chassis/System.Embedded.1"
	glog.Info("fetching server power state")
	body, err := c.fetch(poweredOnPath)
	if err != nil {
		return false, err
	}
	powerState := body["PowerState"]
	return powerState == "On", nil
}

func (c *IDRACClient) AmperageReading() (float64, error) {
	const amperatePath = "/redfish/v1/Chassis/System.Embedded.1/Power/PowerSupplies/PSU.Slot.2"
	glog.Info("fetching voltage state")
	body, err := c.fetch(amperatePath)
	if err != nil {
		return 0, errors.Wrap(err, "failed to fetch amperage from idrac")
	}
	if body["LineInputVoltage"] == nil {
		return 0, nil
	}
	inputVoltage := body["LineInputVoltage"].(float64)
	return inputVoltage, nil
}

func (c *IDRACClient) TurnOff() (map[string]interface{}, error) {
	const managePowerPath = "/redfish/v1/Systems/System.Embedded.1/Actions/ComputerSystem.Reset"
	glog.Warning("powering off server")
	data := []byte("{\"Action\": \"Reset\", \"ResetType\": \"GracefulShutdown\"}")
	response, err := c.post(managePowerPath, data)
	if err != nil {
		return map[string]interface{}{}, errors.Wrap(err, "failed to post turn off command")
	}
	return response, nil
}

func (c *IDRACClient) TurnOn() (map[string]interface{}, error) {
	const managePowerPath = "/redfish/v1/Systems/System.Embedded.1/Actions/ComputerSystem.Reset"
	glog.Warning("powering on server")
	data := []byte("{\"Action\": \"Reset\", \"ResetType\": \"On\"}")
	response, err := c.post(managePowerPath, data)
	if err != nil {
		return map[string]interface{}{}, errors.Wrap(err, "failed to post turn on command")
	}
	return response, nil
}
