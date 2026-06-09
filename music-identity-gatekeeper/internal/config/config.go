package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	DB_URL     string `mapstructure:"DB_URL"`
	REDIS_URL  string `mapstructure:"REDIS_URL"`
	JWT_SECRET string `mapstructure:"JWT_SECRET"`
	PORT       string `mapstructure:"PORT"`
}

func LoadConfig() (Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv() 
	viper.ReadInConfig() 

	var cfg Config
	err := viper.Unmarshal(&cfg)
	return cfg, err
}