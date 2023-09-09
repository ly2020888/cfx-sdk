package main

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Urls        []string `yaml:"urls"`
	Numbers     int      `yaml:"numbers"`
	Rate        int      `yaml:"rate"`
	Concurrency int      `yaml:"concurrency"`
	Time        uint     `yaml:"time"`
	Peers       uint     `yaml:"peers"`
}

func NewConfig() Config {
	viper.SetConfigFile("config.yaml") // 指定配置文件路径
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}
	if err != nil {
		fmt.Println("导出数据有误", err.Error())
	}

	log.Default().Print("配置文件加载成功")

	var _config *Config
	err = viper.Unmarshal(&_config)
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}
	return *_config

}
