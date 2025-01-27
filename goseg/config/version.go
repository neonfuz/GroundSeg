package config

import (
	"encoding/json"
	"fmt"
	"goseg/defaults"
	"goseg/logger"
	"goseg/structs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var (
	VersionServerReady = false
	VersionInfo        structs.Channel
)

// check the version server and return struct
func CheckVersion() (structs.Channel, bool) {
	versMutex.Lock()
	defer versMutex.Unlock()
	conf := Conf()
	releaseChannel := conf.UpdateBranch
	const retries = 10
	const delay = time.Second
	url := globalConfig.UpdateUrl
	var fetchedVersion structs.Version
	for i := 0; i < retries; i++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
		}
		userAgent := "NativePlanet.GroundSeg-" + conf.GsVersion
		req.Header.Set("User-Agent", userAgent)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			errmsg := fmt.Sprintf("Unable to connect to update server: %v", err)
			logger.Logger.Warn(errmsg)
			if i < retries-1 {
				time.Sleep(delay)
				continue
			} else {
				VersionServerReady = false
				return VersionInfo, false
			}
		}
		// read the body bytes
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			errmsg := fmt.Sprintf("Error reading version info: %v", err)
			logger.Logger.Warn(errmsg)
			if i < retries-1 {
				time.Sleep(delay)
				continue
			} else {
				VersionServerReady = false
				return VersionInfo, false
			}
		}
		// unmarshal values into Version struct
		err = json.Unmarshal(body, &fetchedVersion)
		if err != nil {
			errmsg := fmt.Sprintf("Error unmarshalling JSON: %v", err)
			logger.Logger.Warn(errmsg)
			if i < retries-1 {
				time.Sleep(delay)
				continue
			} else {
				VersionServerReady = false
				return VersionInfo, false
			}
		}
		VersionInfo = fetchedVersion.Groundseg[releaseChannel]
		// debug: re-marshal and write the entire fetched version to disk
		confPath := filepath.Join(BasePath, "settings", "version_info.json")
		file, err := os.Create(confPath)
		if err != nil {
			errmsg := fmt.Sprintf("Failed to create file: %v", err)
			logger.Logger.Error(errmsg)
			VersionServerReady = false
			return VersionInfo, false
		}
		defer file.Close()
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "    ")
		if err := encoder.Encode(&fetchedVersion); err != nil {
			errmsg := fmt.Sprintf("Failed to write JSON: %v", err)
			logger.Logger.Error(errmsg)
		}
		VersionServerReady = true
		return VersionInfo, true
	}
	VersionServerReady = false
	return VersionInfo, false
}

// write the defaults.VersionInfo value to disk
func CreateDefaultVersion() error {
	var versionInfo structs.Version
	err := json.Unmarshal([]byte(defaults.DefaultVersionText), &versionInfo)
	if err != nil {
		return err
	}
	prettyJSON, err := json.MarshalIndent(versionInfo, "", "    ")
	if err != nil {
		return err
	}
	filePath := filepath.Join(BasePath, "settings", "version_info.json")
	err = ioutil.WriteFile(filePath, prettyJSON, 0644)
	if err != nil {
		return err
	}
	return nil
}

// return the existing local version info or create default
func LocalVersion() structs.Version {
	confPath := filepath.Join(BasePath, "settings", "version_info.json")
	_, err := os.Open(confPath)
	if err != nil {
		// create a default if it doesn't exist
		err = CreateDefaultVersion()
		if err != nil {
			// panic if we can't create it
			errmsg := fmt.Sprintf("Unable to write version info! %v", err)
			logger.Logger.Error(errmsg)
			panic(errmsg)
		}
	}
	file, err := ioutil.ReadFile(confPath)
	if err != nil {
		errmsg := fmt.Sprintf("Unable to load version info: %v", err)
		panic(errmsg)
	}
	var versionStruct structs.Version
	if err := json.Unmarshal(file, &versionStruct); err != nil {
		errmsg := fmt.Sprintf("Error decoding version JSON: %v", err)
		panic(errmsg)
	}
	return versionStruct
}
