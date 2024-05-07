package config

import (
	"encoding/json"
	"log"
	"os"
)

var dataPath = os.Getenv("HOME") + "/.config/sqltui"
var LogFile = dataPath + "/sqltui.log"
var ConfigFile = dataPath + "/config.yaml"

type MysqlConfig struct {
	UserName string `json:"userName" yaml:"userName"`
	Password string `json:"password" yaml:"password"`
	Host     string `json:"host" yaml:"host"`
	Port     string `json:"port" yaml:"port"`
	DbName   string `json:"dbName" yaml:"dbName"`
}

type RedisConfig struct {
	UserName string `json:"userName" yaml:"userName"`
	Password string `json:"password" yaml:"password"`
	Host     string `json:"host" yaml:"host"`
	Port     string `json:"port" yaml:"port"`
	RdbNum   string `json:"rdbNum" yaml:"rdbNum"`
}

type SqlConfig struct {
	Mysql *MysqlConfig `json:"mysql" yaml:"mysql"`
	Redis *RedisConfig `json:"redis" yaml:"redis"`
}

func init() {
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		log.Fatal(err)
	}

	if err := SetDefaultLog(); err != nil {
		log.Fatal(err)
	}

	if err := SetDefaultConfig(); err != nil {
		log.Fatal(err)
	}
}

func SetDefaultConfig() error {
	if _, err := os.Stat(ConfigFile); err != nil {
		if os.IsNotExist(err) {
			log.Println("ConfigFile not exist, create default config file")

			sqlConfig := SqlConfig{
				Mysql: &MysqlConfig{
					UserName: "root",
					Password: "123456",
					Host:     "127.0.0.1",
					Port:     "3306",
					DbName:   "test_db",
				},
				Redis: &RedisConfig{
					UserName: "",
					Password: "",
					Host:     "127.0.0.1",
					Port:     "6379",
					RdbNum:   "0",
				},
			}

			j, err := json.MarshalIndent(sqlConfig, "", "  ")
			if err != nil {
				return err
			}

			if err := os.WriteFile(ConfigFile, j, 0666); err != nil {
				return err
			}
		}
		return err
	}

	return nil
}

func ReadMySqlConfig() (*MysqlConfig, error) {
	conf, err := os.ReadFile(ConfigFile)
	if err != nil {
		return nil, err
	}
	var tmpConf SqlConfig
	if err := json.Unmarshal(conf, &tmpConf); err != nil {
		return nil, err
	}
	return tmpConf.Mysql, nil
}

func WriteMysqlConfig(mysqlConfig *MysqlConfig) error {
	conf, err := os.ReadFile(ConfigFile)
	if err != nil {
		return err
	}
	var tmpConf SqlConfig
	if err := json.Unmarshal(conf, &tmpConf); err != nil {
		return err
	}
	tmpConf.Mysql = mysqlConfig
	j, err := json.MarshalIndent(tmpConf, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(ConfigFile, j, 0666); err != nil {
		return err
	}
	return nil
}

func ReadRedisConfig() (*RedisConfig, error) {
	conf, err := os.ReadFile(ConfigFile)
	if err != nil {
		return nil, err
	}
	var tmpConf SqlConfig
	if err := json.Unmarshal(conf, &tmpConf); err != nil {
		return nil, err
	}
	return tmpConf.Redis, nil
}

func WriteRedisConfig(redisConfig *RedisConfig) error {
	conf, err := os.ReadFile(ConfigFile)
	if err != nil {
		return err
	}
	var tmpConf SqlConfig
	if err := json.Unmarshal(conf, &tmpConf); err != nil {
		return err
	}
	tmpConf.Redis = redisConfig
	j, err := json.MarshalIndent(tmpConf, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(ConfigFile, j, 0666); err != nil {
		return err
	}
	return nil
}

func SetDefaultLog() error {

	file, err := os.OpenFile(LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	log.SetOutput(file)
	return nil
}
