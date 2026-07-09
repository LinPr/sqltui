package redisbe

import (
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/LinPr/sqltui/internal/db"
)

// KeyTypes lists the key types offered by the key browser, in display order.
var KeyTypes = []string{
	"string",
	"list",
	"set",
	"zset",
	"hash",
	"stream",
}

// Config holds the connection parameters for a redis server. RdbNum is the
// logical database number as entered by the user (must parse as an integer).
type Config struct {
	UserName string
	Password string
	Host     string
	Port     string
	RdbNum   string
}

// Backend adapts the redis data-access layer to the db.KVBackend contract.
type Backend struct {
	rds    *RDS
	addr   string
	rdbNum int
}

var _ db.KVBackend = (*Backend)(nil)

// Connect opens a redis connection and pings it before returning.
func Connect(cfg Config) (*Backend, error) {
	rdbNum, err := strconv.Atoi(cfg.RdbNum)
	if err != nil {
		return nil, fmt.Errorf("invalid redis db number %q: %w", cfg.RdbNum, err)
	}

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	rds, err := NewRDS(&redis.Options{
		Addr:         addr,
		Username:     cfg.UserName,
		Password:     cfg.Password,
		DB:           rdbNum,
		WriteTimeout: 3 * time.Second,
		ReadTimeout:  2 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return &Backend{rds: rds, addr: addr, rdbNum: rdbNum}, nil
}

func (b *Backend) Title() string {
	return fmt.Sprintf("redis://%s/%d", b.addr, b.rdbNum)
}

// Do runs a raw redis command; the reply is rendered as indented JSON.
func (b *Backend) Do(args []string) (string, error) {
	return b.rds.ExecuteRawQuery(args)
}

// ScanKeys lists keys of one type, capped server-side at ScanLimit.
func (b *Backend) ScanKeys(keyType string) ([]string, error) {
	return b.rds.Scan(0, "*", 100, keyType)
}

// Value fetches the value stored at key and renders it per key type.
func (b *Backend) Value(key string) (string, error) {
	return b.rds.GetValue(key)
}

func (b *Backend) Close() error {
	return b.rds.Close()
}

// Close closes the underlying redis client.
func (rds *RDS) Close() error {
	return rds.rdsc.Close()
}
