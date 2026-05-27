package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"sync"

	"restaurants/pkg/logging"
	"restaurants/pkg/sms_sender"
)

type Config struct {
	IsDebug *bool `yaml:"is_debug" env-required:"true"`
	Listen  struct {
		Type   string `yaml:"type" env-default:"port"`
		BindIP string `yaml:"bind_ip" env-default:"0.0.0.0"`
		Port   string `yaml:"port" env-default:"8081"`
	} `yaml:"listen"`
	PublicFilePath string        `yaml:"public_file_path" env-required:"true"`
	Storage        StorageConfig `yaml:"storage"`
	AppVersion     string        `yaml:"app_version" env-required:"true"`
	JwtKey         string        `yaml:"jwt_key" env-required:"true"`
	SecretKey      string        `yaml:"secret_key" env-required:"true"`

	SmsSender sms_sender.Config `yaml:"SMS_SENDER"`

	Redis RedisConfig `yaml:"Redis"`
}

type StorageConfig struct {
	PgPoolMaxConn int    `yaml:"pg_pool_max_conn"`
	Host          string `json:"host"`
	Port          string `json:"port"`
	Database      string `json:"database"`
	Username      string `json:"username"`
	Password      string `json:"password"`
}

type RedisConfig struct {
	Addr            string `yaml:"Addr"`
	Password        string `yaml:"Password"`
	ClientName      string `yaml:"ClientName"`
	MaxRetries      int    `yaml:"MaxRetries"`
	DailTimeout     int    `yaml:"DailTimeout"`
	ReadTimeout     int    `yaml:"ReadTimeout"`
	WriteTimeout    int    `yaml:"WriteTimeout"`
	IdleTimeout     int    `yaml:"IdleTimeout"`
	MaxConnLifeTime int    `yaml:"MaxConnLifeTime"`
	MinIdleConn     int    `yaml:"MinIdleConn"`
	PoolSize        int    `yaml:"PoolSize"`
}

var instance *Config
var once sync.Once

func GetConfig(pathConfig string) *Config {
	once.Do(func() {
		logger := logging.GetLogger()
		logger.Info("read application configuration")
		instance = &Config{}
		if err := cleanenv.ReadConfig(pathConfig, instance); err != nil {
			help, _ := cleanenv.GetDescription(instance, nil)
			logger.Info(help)
			logger.Fatal(err)
		}
	})
	return instance
}
