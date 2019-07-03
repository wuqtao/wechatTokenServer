package config

import (
	"github.com/BurntSushi/toml"
	"io/ioutil"
	"errors"
	"sync"
	"wechatTokenServer/wechat"
)

var configMan ConfigMan
var once sync.Once

type Config struct {
	sync.RWMutex
	Port int
	Wechat []*wechat.WechatConfig
	AheadTime int
	LoopTime int
	LogFile string
	UseIpWhiteList bool
	IpList []string
	AdminIpList []string
	AdminToken string
}

func (conf *Config) GetPort() int{
	defer conf.RUnlock()
	conf.RLock()
	return conf.Port
}

func (conf *Config) GetWechatConfigs() []*wechat.WechatConfig{
	defer conf.RUnlock()
	conf.RLock()
	return conf.Wechat
}

func (conf *Config) GetAheadTime() int{
	defer conf.RUnlock()
	conf.RLock()
	return conf.AheadTime
}

func (conf *Config) GetLoopTime() int{
	defer conf.RUnlock()
	conf.RLock()
	return conf.LoopTime
}

func (conf *Config) GetLogFile() string{
	defer conf.RUnlock()
	conf.RLock()
	return conf.LogFile
}

func (conf *Config) GetIpList() []string{
	defer conf.RUnlock()
	conf.RLock()
	return conf.IpList
}

func (conf *Config) GetAdminIpList() []string{
	defer conf.RUnlock()
	conf.RLock()
	return conf.AdminIpList
}

func (conf *Config) GetAdminToken() string{
	defer conf.RUnlock()
	conf.RLock()
	return conf.AdminToken
}

type ConfigMan struct {
	sync.RWMutex
	config *Config
}

func (cm *ConfigMan) GetConfig() *Config{
	return cm.config
}

func (cm *ConfigMan) SetConfig(newConfig *Config){
	cm.Lock()
	cm.config = newConfig
	cm.Unlock()
}

func GetConfigMan() *ConfigMan{
	once.Do(func() {
		configMan = ConfigMan{}
	})
	return &configMan
}

func LoadConfig(configFile string) (*Config,error){
	config := Config{}
	fileContent,err := ioutil.ReadFile(configFile)
	if err != nil{
		return nil,err
	}
	if _,err := toml.Decode(string(fileContent),&config);err != nil{
		return nil,err
	}
	config.Lock()
	if config.LoopTime <= 0{
		return nil,errors.New("looptime must be great than 0")
	}
	if config.AheadTime >= 7200 || config.AheadTime < 0{
		return nil,errors.New("aheadTime must be between 0 and 7200")
	}
	if config.Port <= 0{
		return nil,errors.New("port must be great than 0")
	}

	if len(config.Wechat) == 0{
		return nil,errors.New("must config one or more wechat info")
	}

	if config.UseIpWhiteList && (len(config.IpList) == 0 || config.IpList == nil){
		config.IpList = append(config.IpList,"127.0.0.1")
	}

	if len(config.AdminIpList) == 0{
		config.AdminIpList = append(config.AdminIpList,"127.0.0.1")
	}
	config.Unlock()
	return &config,nil
}

