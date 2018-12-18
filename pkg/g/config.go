package g

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
	"github.com/urfave/cli"
)

// Configuration are the available config values
type Configuration struct {
	ListenAddr      string      `yaml:"ListenAddr" env:"LISTEN_ADDR"`
	DataDir         string      `yaml:"DataDir" env:"DATA_DIR"`
	Backend         string      `yaml:"Backend" env:"BACKEND"`
	Location        string      `yaml:"Location" env:"LOCATION"`
	ShortedIDLength int         `yaml:"ShortedIDLength" env:"SHORTED_ID_LENGTH"`
	Redis           redisConfig `yaml:"Redis" env:"REDIS"`
	Log             LogConfig   `yaml:"Log" env:"LOG"`
}

type redisConfig struct {
	Host         string `yaml:"Host" env:"HOST"`
	Password     string `yaml:"Password" env:"PASSWORD"`
	DB           int    `yaml:"DB" env:"DB"`
	MaxRetries   int    `yaml:"MaxRetries" env:"MAX_RETRIES"`
	ReadTimeout  string `yaml:"ReadTimeout" env:"READ_TIMEOUT"`
	WriteTimeout string `yaml:"WriteTimeout" env:"WRITE_TIMEOUT"`
	SessionDB    string `yaml:"SessionDB" env:"SESSION_DB"`
	SharedKey    string `yaml:"SharedKey" env:"SHARED_KEY"`
}

type LogConfig struct {
	Dir   string `yaml:"Dir" env:"Dir"`
	Level string `yaml:"Level" env:"Level"`
}

// Config contains the default values
var (
	config = Configuration{
		ListenAddr:      ":8080",
		DataDir:         "data",
		Backend:         "redis",
		Location:        "",
		ShortedIDLength: 4,
		Redis: redisConfig{
			Host:         "127.0.0.1:6379",
			MaxRetries:   3,
			ReadTimeout:  "3s",
			WriteTimeout: "3s",
			SessionDB:    "1",
			SharedKey:    "secret",
		},
	}

	lock = new(sync.RWMutex)
)

func ReadInConfig(ctx *cli.Context) error {
	viper.SetConfigName("config")
	viper.AddConfigPath(ctx.String("config"))

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("Fatal error config file: %s \n", err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		return fmt.Errorf("Fatal error config file: %s \n", err)
	}

	return nil
}

// GetConfig returns the configuration from the memory
func GetConfig() Configuration {
	lock.RLock()
	defer lock.RUnlock()

	return config
}

// SetConfig sets the configuration into the memory
func SetConfig(_config Configuration) {
	config = _config
}
