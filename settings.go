package git2neo4j

import (
	"bytes"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

var buffer bytes.Buffer
var settings = Settings{}
var env = "test"

// Settings contains the information regarding folder paths and databases.
type Settings struct {
	// Url to acces neo4j. Basic auth can be added here.
	// E.g: http://user:password@localhost:7474
	DbUrl string
	// Check if setting is already set.
	isset bool
}

// GetSettings gets and return Settings according to the configuration files
func GetSettings() Settings {
	if !settings.isset {
		initSettings()
	}
	return settings
}

// InitSettings get the environments value from flags, default fall back is test.
func initSettings() {
	env = os.Getenv("GO_ENV")
	if env == "" {
		fmt.Println("Warning: Setting environment as development due to lack of GO_ENV value")
		env = "development"
	}
	loadSettingsByEnv(env)
}

// LoadSettingsByEnv get environment dependent values.
func loadSettingsByEnv(env string) {
	if env == "test" {
		buffer.WriteString("test_dir/")
	}
	buffer.WriteString("config.toml")
	if _, err := toml.DecodeFile(buffer.String(), &settings); err != nil {
		panic(err)
	}
	settings.isset = true
}
