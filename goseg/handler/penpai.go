package handler

import (
	"encoding/json"
	"fmt"
	"goseg/config"
	"goseg/docker"
	"goseg/logger"
	"goseg/structs"
	"runtime"
)

func PenpaiHandler(msg []byte) error {
	logger.Logger.Info("Penpai")
	var penpaiPayload structs.WsPenpaiPayload
	err := json.Unmarshal(msg, &penpaiPayload)
	if err != nil {
		return fmt.Errorf("Couldn't unmarshal penpai payload: %v", err)
	}
	conf := config.Conf()
	switch penpaiPayload.Payload.Action {
	case "toggle":
		running := false
		if conf.PenpaiRunning {
			// stop container
			err := docker.StopContainerByName("llama-gpt-api")
			if err != nil {
				return fmt.Errorf(fmt.Sprintf("Failed to stop Llama API: %v", err))
			}
			err = docker.StopContainerByName("llama-gpt-ui")
			if err != nil {
				return fmt.Errorf(fmt.Sprintf("Failed to stop Llama UI: %v", err))
			}
		} else {
			// start container
			info, err := docker.StartContainer("llama-gpt-api", "llama-api")
			if err != nil {
				return fmt.Errorf(fmt.Sprintf("Error starting Llama API: %v", err))
			}
			config.UpdateContainerState("llama-api", info)
			info, err = docker.StartContainer("llama-gpt-ui", "llama-ui")
			if err != nil {
				return fmt.Errorf(fmt.Sprintf("Error starting Llama UI: %v", err))
			}
			config.UpdateContainerState("llama-ui", info)
			running = true
		}
		if err = config.UpdateConf(map[string]interface{}{
			"penpaiRunning": running,
		}); err != nil {
			return fmt.Errorf(fmt.Sprintf("%v", err))
		}
		return nil
	case "set-model":
		// update config
		model := penpaiPayload.Payload.Model
		if err = config.UpdateConf(map[string]interface{}{
			"penpaiActive": model,
		}); err != nil {
			return fmt.Errorf(fmt.Sprintf("%v", err))
		}
		if err := docker.DeleteContainer("llama-gpt-api"); err != nil {
			return fmt.Errorf("Failed to delete container: %v", err)
		}
		// if running, restart container
		if conf.PenpaiRunning {
			if _, err := docker.StartContainer("llama-gpt-api", "llama-api"); err != nil {
				return fmt.Errorf("Couldn't start Llama API: %v", err)
			}
		}
	case "set-cores":
		cores := penpaiPayload.Payload.Cores
		// check if core count is valid
		if cores < 1 {
			return fmt.Errorf("Penpai unable to set 0 cores!")
		}
		if cores >= runtime.NumCPU() {
			return fmt.Errorf(fmt.Sprintf("Penpai unable to set %v cores!", cores))
		}
		// update config
		if err = config.UpdateConf(map[string]interface{}{
			"penpaiCores": cores,
		}); err != nil {
			return fmt.Errorf(fmt.Sprintf("%v", err))
		}
		if err := docker.DeleteContainer("llama-gpt-api"); err != nil {
			return fmt.Errorf("Failed to delete container: %v", err)
		}
		// if running, restart container
		if conf.PenpaiRunning {
			if _, err := docker.StartContainer("llama-gpt-api", "llama-api"); err != nil {
				return fmt.Errorf("Couldn't start Llama API: %v", err)
			}
		}
		return nil
	case "remove":
		// check if container exists
		// remove container, delete volume
		logger.Logger.Debug(fmt.Sprintf("Todo: remove penpai"))
	}
	return nil
}
