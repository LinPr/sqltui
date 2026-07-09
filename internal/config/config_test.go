package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// useTempConfig points the package at a config file inside a temp dir
// (standing in for a temp HOME) and restores the original paths afterwards.
func useTempConfig(t *testing.T) string {
	t.Helper()

	oldDataPath, oldConfigFile := dataPath, ConfigFile
	dir := t.TempDir()
	dataPath = dir
	ConfigFile = filepath.Join(dir, "config.yaml")
	t.Cleanup(func() {
		dataPath = oldDataPath
		ConfigFile = oldConfigFile
	})
	return dir
}

func TestUIConfigRoundtrip(t *testing.T) {
	useTempConfig(t)

	want := &UIConfig{Theme: "nord", ShowBorders: false, ShowRowNumbers: true}
	if err := WriteUIConfig(want); err != nil {
		t.Fatalf("WriteUIConfig: %v", err)
	}

	got, err := ReadUIConfig()
	if err != nil {
		t.Fatalf("ReadUIConfig: %v", err)
	}
	if *got != *want {
		t.Fatalf("roundtrip mismatch: got %+v, want %+v", got, want)
	}

	info, err := os.Stat(ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("config file perm = %o, want 600", perm)
	}
}

func TestWritersProduceYAML(t *testing.T) {
	useTempConfig(t)

	if err := WriteUIConfig(&UIConfig{Theme: "nord", ShowBorders: true, ShowRowNumbers: true}); err != nil {
		t.Fatalf("WriteUIConfig: %v", err)
	}

	raw, err := os.ReadFile(ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	// valid YAML mapping, not JSON
	var asYAML map[string]any
	if err := yaml.Unmarshal(raw, &asYAML); err != nil {
		t.Fatalf("config file is not valid yaml: %v\n%s", err, raw)
	}
	if _, ok := asYAML["ui"]; !ok {
		t.Fatalf("yaml config missing ui section:\n%s", raw)
	}
	var asJSON map[string]any
	if err := json.Unmarshal(raw, &asJSON); err == nil {
		t.Fatalf("config file still looks like json:\n%s", raw)
	}
}

func TestReadUIConfigDefaultsWhenSectionMissing(t *testing.T) {
	useTempConfig(t)

	// write a config that has no ui section
	if err := WriteMysqlConfig(&MysqlConfig{UserName: "u", Host: "h", Port: "1", DbName: "d"}); err != nil {
		t.Fatalf("WriteMysqlConfig: %v", err)
	}

	got, err := ReadUIConfig()
	if err != nil {
		t.Fatalf("ReadUIConfig: %v", err)
	}
	want := DefaultUIConfig()
	if *got != *want {
		t.Fatalf("defaults mismatch: got %+v, want %+v", got, want)
	}
}

func TestReadUIConfigMissingFile(t *testing.T) {
	useTempConfig(t)

	if _, err := ReadUIConfig(); err == nil {
		t.Fatal("expected error when config file is missing")
	}
}

func TestWriteUIConfigPreservesOtherSections(t *testing.T) {
	useTempConfig(t)

	mysql := &MysqlConfig{UserName: "root", Password: "pw", Host: "127.0.0.1", Port: "3306", DbName: "db"}
	if err := WriteMysqlConfig(mysql); err != nil {
		t.Fatalf("WriteMysqlConfig: %v", err)
	}
	if err := WriteUIConfig(&UIConfig{Theme: "dracula", ShowBorders: true, ShowRowNumbers: false}); err != nil {
		t.Fatalf("WriteUIConfig: %v", err)
	}

	gotMysql, err := ReadMySqlConfig()
	if err != nil {
		t.Fatalf("ReadMySqlConfig: %v", err)
	}
	if *gotMysql != *mysql {
		t.Fatalf("mysql section clobbered: got %+v, want %+v", gotMysql, mysql)
	}

	gotUI, err := ReadUIConfig()
	if err != nil {
		t.Fatalf("ReadUIConfig: %v", err)
	}
	if gotUI.Theme != "dracula" || !gotUI.ShowBorders || gotUI.ShowRowNumbers {
		t.Fatalf("unexpected ui section: %+v", gotUI)
	}
}

// writeLegacyJSON drops a JSON config (the pre-YAML format) into the temp
// config dir.
func writeLegacyJSON(t *testing.T, dir string, conf SqlConfig) string {
	t.Helper()
	j, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(dir, "config.json")
	if err := os.WriteFile(legacy, j, 0600); err != nil {
		t.Fatal(err)
	}
	return legacy
}

func TestMigrateLegacyJSONOnRead(t *testing.T) {
	dir := useTempConfig(t)

	legacyConf := SqlConfig{
		Mysql: &MysqlConfig{UserName: "legacy", Password: "pw", Host: "10.0.0.1", Port: "3307", DbName: "old"},
		UI:    &UIConfig{Theme: "gruvbox", ShowBorders: false, ShowRowNumbers: true},
	}
	legacy := writeLegacyJSON(t, dir, legacyConf)
	legacyBefore, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}

	// a plain read triggers the migration and sees the legacy values
	gotMysql, err := ReadMySqlConfig()
	if err != nil {
		t.Fatalf("ReadMySqlConfig: %v", err)
	}
	if *gotMysql != *legacyConf.Mysql {
		t.Fatalf("migrated mysql mismatch: got %+v, want %+v", gotMysql, legacyConf.Mysql)
	}
	gotUI, err := ReadUIConfig()
	if err != nil {
		t.Fatalf("ReadUIConfig: %v", err)
	}
	if *gotUI != *legacyConf.UI {
		t.Fatalf("migrated ui mismatch: got %+v, want %+v", gotUI, legacyConf.UI)
	}

	// the yaml file now exists with tight permissions...
	info, err := os.Stat(ConfigFile)
	if err != nil {
		t.Fatalf("config.yaml not created by migration: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("migrated config perm = %o, want 600", perm)
	}

	// ...and the json file is untouched
	legacyAfter, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatalf("legacy json removed by migration: %v", err)
	}
	if string(legacyAfter) != string(legacyBefore) {
		t.Fatal("legacy json modified by migration")
	}
}

func TestMigrationSkippedWhenYAMLExists(t *testing.T) {
	dir := useTempConfig(t)

	if err := WriteUIConfig(&UIConfig{Theme: "nord", ShowBorders: true, ShowRowNumbers: true}); err != nil {
		t.Fatalf("WriteUIConfig: %v", err)
	}
	writeLegacyJSON(t, dir, SqlConfig{
		UI: &UIConfig{Theme: "gruvbox", ShowBorders: false, ShowRowNumbers: false},
	})

	got, err := ReadUIConfig()
	if err != nil {
		t.Fatalf("ReadUIConfig: %v", err)
	}
	if got.Theme != "nord" {
		t.Fatalf("yaml config overridden by stale json: %+v", got)
	}
}

func TestSetDefaultConfigMigratesLegacyJSON(t *testing.T) {
	dir := useTempConfig(t)

	writeLegacyJSON(t, dir, SqlConfig{
		Postgres: &PostgresConfig{UserName: "u", Host: "h", Port: "5433", DbName: "d", SslMode: "require"},
	})

	if err := SetDefaultConfig(); err != nil {
		t.Fatalf("SetDefaultConfig: %v", err)
	}

	got, err := ReadPostgresConfig()
	if err != nil {
		t.Fatalf("ReadPostgresConfig: %v", err)
	}
	if got.Port != "5433" || got.SslMode != "require" {
		t.Fatalf("legacy postgres values lost in migration: %+v", got)
	}
}

func TestSetDefaultConfigCreatesYAMLDefaults(t *testing.T) {
	useTempConfig(t)

	if err := SetDefaultConfig(); err != nil {
		t.Fatalf("SetDefaultConfig: %v", err)
	}

	if filepath.Ext(ConfigFile) != ".yaml" {
		t.Fatalf("canonical config file is not yaml: %s", ConfigFile)
	}
	got, err := ReadUIConfig()
	if err != nil {
		t.Fatalf("ReadUIConfig: %v", err)
	}
	want := DefaultUIConfig()
	if *got != *want {
		t.Fatalf("default ui mismatch: got %+v, want %+v", got, want)
	}
}

func TestMigrationIgnoresCorruptLegacyJSON(t *testing.T) {
	dir := useTempConfig(t)

	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := SetDefaultConfig(); err != nil {
		t.Fatalf("SetDefaultConfig with corrupt legacy json: %v", err)
	}
	got, err := ReadUIConfig()
	if err != nil {
		t.Fatalf("ReadUIConfig: %v", err)
	}
	want := DefaultUIConfig()
	if *got != *want {
		t.Fatalf("defaults mismatch after corrupt migration: got %+v, want %+v", got, want)
	}
}
