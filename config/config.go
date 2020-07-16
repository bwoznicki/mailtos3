package config

import (
	"encoding/json"
	"os"
	"strings"

	"mailtos3/logger"
	"mailtos3/sysexits"
)

// Config is the main app config
type Config struct {
	RequestConfig RequestConfig `json:"requestConfig"`
	Mailboxes     []Mailbox     `json:"mailboxes"`
}

// RequestConfig holds the configuration for the request to aws api
type RequestConfig struct {
	Region   string `json:"region"`
	Timeout  int    `json:"timeout"`
	Endpoint bool   `json:"endpoint"`
}

// Mailbox hold the configuration for individual mailbox settings
type Mailbox struct {
	Address   string `json:"address"`
	Bucket    string `json:"bucket"`
	CmkKeyArn string `json:"cmkKeyArn"`
}

// Load loads the local config
func Load() Config {

	var config Config
	var configFile *os.File
	var err error

	if configFile, err = os.Open("/usr/local/bin/mailtos3/config.json"); err != nil {
		// if config not fund under /usr/local/bin/ try current working dir
		dir, _ := os.Getwd()
		configFile, err = os.Open(dir + "/config.json")
	}
	defer configFile.Close()

	if err != nil {
		logger.Log.Printf("[ERROR] Unable to load config file, %s", err)
		// let mta know that there is a problem with configuration
		os.Exit(sysexits.EX_CONFIG)
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		logger.Log.Printf("[ERROR] Invalid syntax in config file: %s", err)
		// let mta know that there is a problem with configuration
		os.Exit(sysexits.EX_CONFIG)
	}

	if dupes, found := checkDuplicates(config.Mailboxes); found {
		logger.Log.Printf("[WARNING] Duplicate mailbox configuration found for user(s): %s. "+
			"Only the first configured mailbox will be matched.", *dupes)
	}
	return config
}

func checkDuplicates(mailboxes []Mailbox) (*string, bool) {
	// iterate over mailboxes and locate blocks
	// with duplicate email address
	var users, dupes string
	for _, m := range mailboxes {
		u := strings.ToLower(m.Address)
		if strings.Contains(users, u) {
			dupes += ", " + u
		} else {
			users += " " + u
		}
	}
	dupes = strings.TrimPrefix(dupes, ", ")
	if len(dupes) > 0 {
		return &dupes, true
	}
	return nil, false

}
