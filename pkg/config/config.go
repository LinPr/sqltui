package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

var dataPath = os.Getenv("HOME") + "/.config/sqltui"
var LogFile = dataPath + "/sqltui.log"
var ConfigFile = dataPath + "/config.json"

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
type SqliteConfig struct {
	FilePath string `json:"filePath" yaml:"filePath"`
}

type PostgresConfig struct {
	UserName string `json:"userName" yaml:"userName"`
	Password string `json:"password" yaml:"password"`
	Host     string `json:"host" yaml:"host"`
	Port     string `json:"port" yaml:"port"`
	DbName   string `json:"dbName" yaml:"dbName"`
	SslMode  string `json:"sslMode" yaml:"sslMode"`
}

type SqlConfig struct {
	Mysql    *MysqlConfig    `json:"mysql" yaml:"mysql"`
	Redis    *RedisConfig    `json:"redis" yaml:"redis"`
	Sqlite   *SqliteConfig   `json:"sqlite" yaml:"sqlite"`
	Postgres *PostgresConfig `json:"postgres" yaml:"postgres"`
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
				Sqlite: &SqliteConfig{
					FilePath: filepath.Join(dataPath, "sqlite.default"),
				},
				Postgres: &PostgresConfig{
					UserName: "postgres",
					Password: "",
					Host:     "127.0.0.1",
					Port:     "5432",
					DbName:   "postgres",
					SslMode:  "disable",
				},
			}

			j, err := json.MarshalIndent(sqlConfig, "", "  ")
			if err != nil {
				return err
			}

			if err := os.WriteFile(ConfigFile, j, 0600); err != nil {
				return err
			}
			return nil
		}
		return err
	}

	// tighten the permissions of a pre-existing config file, since it
	// stores database passwords in plaintext
	return os.Chmod(ConfigFile, 0600)
}

// readSqlConfig reads and parses the config file. When the file is missing
// or corrupt it returns a zero SqlConfig so that saving still works.
func readSqlConfig() SqlConfig {
	var tmpConf SqlConfig
	conf, err := os.ReadFile(ConfigFile)
	if err != nil {
		return SqlConfig{}
	}
	if err := json.Unmarshal(conf, &tmpConf); err != nil {
		return SqlConfig{}
	}
	return tmpConf
}

func writeSqlConfig(tmpConf SqlConfig) error {
	j, err := json.MarshalIndent(tmpConf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFile, j, 0600)
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
	if tmpConf.Mysql == nil {
		tmpConf.Mysql = &MysqlConfig{
			UserName: "root",
			Host:     "127.0.0.1",
			Port:     "3306",
			DbName:   "test_db",
		}
	}
	return tmpConf.Mysql, nil
}

func WriteMysqlConfig(mysqlConfig *MysqlConfig) error {
	tmpConf := readSqlConfig()
	tmpConf.Mysql = mysqlConfig
	return writeSqlConfig(tmpConf)
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
	if tmpConf.Redis == nil {
		tmpConf.Redis = &RedisConfig{
			Host:   "127.0.0.1",
			Port:   "6379",
			RdbNum: "0",
		}
	}
	return tmpConf.Redis, nil
}

func WriteRedisConfig(redisConfig *RedisConfig) error {
	tmpConf := readSqlConfig()
	tmpConf.Redis = redisConfig
	return writeSqlConfig(tmpConf)
}

func ReadSqliteConfig() (*SqliteConfig, error) {
	conf, err := os.ReadFile(ConfigFile)
	if err != nil {
		return nil, err
	}
	var tmpConf SqlConfig
	if err := json.Unmarshal(conf, &tmpConf); err != nil {
		return nil, err
	}
	if tmpConf.Sqlite == nil {
		tmpConf.Sqlite = &SqliteConfig{
			FilePath: filepath.Join(dataPath, "sqlite.default"),
		}
	}
	return tmpConf.Sqlite, nil
}

func WriteSqliteConfig(sqliteConfig *SqliteConfig) error {
	tmpConf := readSqlConfig()
	tmpConf.Sqlite = sqliteConfig
	return writeSqlConfig(tmpConf)
}

func ReadPostgresConfig() (*PostgresConfig, error) {
	conf, err := os.ReadFile(ConfigFile)
	if err != nil {
		return nil, err
	}
	var tmpConf SqlConfig
	if err := json.Unmarshal(conf, &tmpConf); err != nil {
		return nil, err
	}
	if tmpConf.Postgres == nil {
		tmpConf.Postgres = &PostgresConfig{
			UserName: "postgres",
			Host:     "127.0.0.1",
			Port:     "5432",
			DbName:   "postgres",
			SslMode:  "disable",
		}
	}
	return tmpConf.Postgres, nil
}

func WritePostgresConfig(postgresConfig *PostgresConfig) error {
	tmpConf := readSqlConfig()
	tmpConf.Postgres = postgresConfig
	return writeSqlConfig(tmpConf)
}

func SetDefaultLog() error {
	// truncate an oversized log at startup so it does not grow without bound
	const maxLogSize = 1 << 20 // 1 MB
	flag := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if info, err := os.Stat(LogFile); err == nil && info.Size() > maxLogSize {
		flag = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}

	file, err := os.OpenFile(LogFile, flag, 0644)
	if err != nil {
		return err
	}

	log.SetOutput(file)
	return nil
}
