package handler

import (
	"encoding/json"
	"fmt"
	"goseg/config"
	"goseg/docker"
	"goseg/logger"
	"goseg/structs"
	"goseg/system"
	"os"
	"os/exec"
	"time"
)

// handle system events
func SystemHandler(msg []byte) error {
	logger.Logger.Info("System")
	var systemPayload structs.WsSystemPayload
	err := json.Unmarshal(msg, &systemPayload)
	if err != nil {
		return fmt.Errorf("Couldn't unmarshal system payload: %v", err)
	}
	switch systemPayload.Payload.Action {
	case "toggle-penpai-feature":
		conf := config.Conf()
		if conf.PenpaiAllow {
			err := docker.StopContainerByName("llama-gpt-api")
			if err != nil {
				logger.Logger.Error(fmt.Sprintf("Failed to stop Llama API: %v", err))
			}
			err = docker.StopContainerByName("llama-gpt-ui")
			if err != nil {
				logger.Logger.Error(fmt.Sprintf("Failed to stop Llama UI: %v", err))
			}
			if err = config.UpdateConf(map[string]interface{}{
				"penpaiAllow": false,
			}); err != nil {
				logger.Logger.Error(fmt.Sprintf("Couldn't toggle penpai feature: %v", err))
			}
		} else {
			if err = config.UpdateConf(map[string]interface{}{
				"penpaiAllow": true,
			}); err != nil {
				logger.Logger.Error(fmt.Sprintf("Couldn't toggle penpai feature: %v", err))
			}
			if err := docker.LoadLlama(); err != nil {
				logger.Logger.Error(fmt.Sprintf("Failed to load llama docker: %v", err))
			}
		}
	case "groundseg":
		logger.Logger.Info(fmt.Sprintf("Device shutdown requested"))
		switch systemPayload.Payload.Command {
		case "restart":
			if config.DebugMode {
				logger.Logger.Debug(fmt.Sprintf("DebugMode detected, skipping GroundSeg restart. Exiting program."))
				os.Exit(0)
			} else {
				logger.Logger.Info(fmt.Sprintf("Restarting GroundSeg.."))
				cmd := exec.Command("systemctl", "restart", "groundseg")
				cmd.Run()
			}
		default:
			return fmt.Errorf("Unrecognized groundseg.service command: %v", systemPayload.Payload.Command)
		}
	case "power":
		switch systemPayload.Payload.Command {
		case "shutdown":
			logger.Logger.Info(fmt.Sprintf("Device shutdown requested"))
			if config.DebugMode {
				logger.Logger.Debug(fmt.Sprintf("DebugMode detected, skipping shutdown. Exiting program."))
				os.Exit(0)
			} else {
				logger.Logger.Info(fmt.Sprintf("Turning off device.."))
				cmd := exec.Command("shutdown", "-h", "now")
				cmd.Run()
			}
		case "restart":
			logger.Logger.Info(fmt.Sprintf("Device restart requested"))
			if config.DebugMode {
				logger.Logger.Debug(fmt.Sprintf("DebugMode detected, skipping restart. Exiting program."))
				os.Exit(0)
			} else {
				logger.Logger.Info(fmt.Sprintf("Restarting device.."))
				cmd := exec.Command("reboot")
				cmd.Run()
			}
		default:
			return fmt.Errorf("Unrecognized power command: %v", systemPayload.Payload.Command)
		}
	case "modify-swap":
		logger.Logger.Info(fmt.Sprintf("Updating swap with value %v", systemPayload.Payload.Value))
		//broadcast.SysTransBus <- structs.SystemTransition{Swap: true, Type: "swap"}
		conf := config.Conf()
		file := conf.SwapFile
		if err := system.ConfigureSwap(file, systemPayload.Payload.Value); err != nil {
			logger.Logger.Error(fmt.Sprintf("Unable to set swap: %v", err))
			//broadcast.SysTransBus <- structs.SystemTransition{Swap: false, Type: "swap"}
			return fmt.Errorf("Unable to set swap: %v", err)
		}
		if err = config.UpdateConf(map[string]interface{}{
			"swapVal": systemPayload.Payload.Value,
		}); err != nil {
			logger.Logger.Error(fmt.Sprintf("Couldn't update swap value: %v", err))
		}
		go func() {
			time.Sleep(2 * time.Second)
			//broadcast.SysTransBus <- structs.SystemTransition{Swap: false, Type: "swap"}
		}()
		logger.Logger.Info(fmt.Sprintf("Swap successfully set to %v", systemPayload.Payload.Value))
	case "update":
		if systemPayload.Payload.Update == "linux" {
			if err := system.RunUpgrade(); err != nil {
				logger.Logger.Error(fmt.Sprintf("Error updating host system: %v", err))
			}
		}
	case "wifi-toggle":
		if err := system.ToggleDevice(system.Device); err != nil {
			logger.Logger.Error(fmt.Sprintf("Couldn't toggle wifi device: %v", err))
		}
	case "wifi-connect":
		docker.SysTransBus <- structs.SystemTransition{Type: "wifiConnect", Event: "connecting"}
		if err := system.ConnectToWifi(systemPayload.Payload.SSID, systemPayload.Payload.Password); err != nil {
			docker.SysTransBus <- structs.SystemTransition{Type: "wifiConnect", Event: "error"}
			time.Sleep(3 * time.Second)
			docker.SysTransBus <- structs.SystemTransition{Type: "wifiConnect", Event: ""}
			return fmt.Errorf("Couldn't connect to wifi: %v", err)
		}
		docker.SysTransBus <- structs.SystemTransition{Type: "wifiConnect", Event: "success"}
		time.Sleep(3 * time.Second)
		docker.SysTransBus <- structs.SystemTransition{Type: "wifiConnect", Event: ""}
	default:
		return fmt.Errorf("Unrecognized system action: %v", systemPayload.Payload.Action)
	}
	return nil
}
