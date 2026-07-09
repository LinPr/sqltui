package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var dataPath = os.Getenv("HOME") + "/.config/sqltui"
var LogFile = dataPath + "/sqltui.log"

// ConfigFile is the canonical on-disk config location (YAML). Older versions
// stored JSON in config.json next to it; that file is migrated automatically
// on load (see migrateLegacyConfig) and left in place.
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

// UIConfig holds viewer preferences persisted between runs.
type UIConfig struct {
	Theme          string `json:"theme" yaml:"theme"`
	ShowBorders    bool   `json:"showBorders" yaml:"showBorders"`
	ShowRowNumbers bool   `json:"showRowNumbers" yaml:"showRowNumbers"`
}

// DefaultUIConfig returns the UI preferences used when none are saved.
func DefaultUIConfig() *UIConfig {
	return &UIConfig{
		Theme:          "sorbet",
		ShowBorders:    true,
		ShowRowNumbers: true,
	}
}

type SqlConfig struct {
	Mysql    *MysqlConfig    `json:"mysql" yaml:"mysql"`
	Redis    *RedisConfig    `json:"redis" yaml:"redis"`
	Sqlite   *SqliteConfig   `json:"sqlite" yaml:"sqlite"`
	Postgres *PostgresConfig `json:"postgres" yaml:"postgres"`
	UI       *UIConfig       `json:"ui" yaml:"ui"`
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

func defaultSqlConfig() SqlConfig {
	return SqlConfig{
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
		UI: DefaultUIConfig(),
	}
}

// legacyConfigFile is the pre-YAML JSON config location, derived from the
// current ConfigFile so tests that repoint ConfigFile stay self-contained.
func legacyConfigFile() string {
	return filepath.Join(filepath.Dir(ConfigFile), "config.json")
}

// migrateLegacyConfig converts an old JSON config to the YAML file when the
// YAML file does not exist yet. The JSON file is left in place untouched.
// A missing JSON file is not an error; an unreadable one is only logged so
// that a fresh default config can still be created.
func migrateLegacyConfig() error {
	legacy := legacyConfigFile()
	if legacy == ConfigFile {
		return nil
	}
	if _, err := os.Stat(ConfigFile); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	raw, err := os.ReadFile(legacy)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var tmpConf SqlConfig
	if err := json.Unmarshal(raw, &tmpConf); err != nil {
		log.Printf("config: cannot migrate %s (%v); starting from defaults", legacy, err)
		return nil
	}
	if err := writeSqlConfig(tmpConf); err != nil {
		return fmt.Errorf("config: migrating %s: %w", legacy, err)
	}
	log.Printf("config: migrated %s to %s (the old JSON file was left in place)", legacy, ConfigFile)
	return nil
}

// SetDefaultConfig makes sure a config file exists: it migrates a legacy JSON
// config if present, creates a default YAML file otherwise, and tightens the
// permissions of a pre-existing file.
func SetDefaultConfig() error {
	if err := migrateLegacyConfig(); err != nil {
		return err
	}

	if _, err := os.Stat(ConfigFile); err != nil {
		if os.IsNotExist(err) {
			log.Println("config file not found, creating default", ConfigFile)
			return writeSqlConfig(defaultSqlConfig())
		}
		return err
	}

	// tighten the permissions of a pre-existing config file, since it
	// stores database passwords in plaintext
	return os.Chmod(ConfigFile, 0600)
}

// readConfig migrates any legacy JSON config, then reads and parses the YAML
// config file.
func readConfig() (SqlConfig, error) {
	var tmpConf SqlConfig
	if err := migrateLegacyConfig(); err != nil {
		return tmpConf, err
	}
	raw, err := os.ReadFile(ConfigFile)
	if err != nil {
		return tmpConf, err
	}
	if err := yaml.Unmarshal(raw, &tmpConf); err != nil {
		return tmpConf, err
	}
	return tmpConf, nil
}

// readSqlConfig reads and parses the config file. When the file is missing
// or corrupt it returns a zero SqlConfig so that saving still works.
func readSqlConfig() SqlConfig {
	tmpConf, err := readConfig()
	if err != nil {
		return SqlConfig{}
	}
	return tmpConf
}

func writeSqlConfig(tmpConf SqlConfig) error {
	y, err := yaml.Marshal(&tmpConf)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFile, y, 0600)
}

func ReadMySqlConfig() (*MysqlConfig, error) {
	tmpConf, err := readConfig()
	if err != nil {
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
	tmpConf, err := readConfig()
	if err != nil {
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
	tmpConf, err := readConfig()
	if err != nil {
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
	tmpConf, err := readConfig()
	if err != nil {
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

func ReadUIConfig() (*UIConfig, error) {
	tmpConf, err := readConfig()
	if err != nil {
		return nil, err
	}
	if tmpConf.UI == nil {
		tmpConf.UI = DefaultUIConfig()
	}
	return tmpConf.UI, nil
}

func WriteUIConfig(uiConfig *UIConfig) error {
	tmpConf := readSqlConfig()
	tmpConf.UI = uiConfig
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
