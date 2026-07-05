package redis

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

var RdsClinet *RDS

// max keys collected when browsing a key type from the tree view
const ScanLimit = 500

type RDS struct {
	rdsc *redis.Client
}

func NewRDS(rdsOpt *redis.Options) (*RDS, error) {
	rdsc := redis.NewClient(rdsOpt)

	if _, err := rdsc.Ping(context.Background()).Result(); err != nil {
		rdsc.Close()
		return nil, err
	}

	// close the previous connection when re-connecting
	if RdsClinet != nil {
		RdsClinet.rdsc.Close()
	}

	RdsClinet = &RDS{
		rdsc: rdsc,
	}
	return RdsClinet, nil
}

func (rds *RDS) ExecuteRawQuery(args []string) (string, error) {
	var tmpArgs []any
	for _, arg := range args {
		tmpArgs = append(tmpArgs, arg)
	}

	cmd := rds.rdsc.Do(context.Background(), tmpArgs...)
	result, err := cmd.Result()
	if err != nil {
		return "", err
	}

	return FormatJson(result)
}

func (rds *RDS) Scan(cursor uint64, match string, count int64, keyType string) ([]string, error) {
	iter := rds.rdsc.ScanType(context.Background(), cursor, match, count, keyType).Iterator()
	var keys []string
	for iter.Next(context.Background()) {
		keys = append(keys, iter.Val())
		if len(keys) >= ScanLimit {
			// cap the number of keys so a huge keyspace does not
			// freeze the UI or build an unbounded tree
			break
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (rds *RDS) GetValue(key string) (string, error) {
	keyType := rds.rdsc.Type(context.Background(), key).Val()
	switch keyType {
	case "string":
		val, err := rds.rdsc.Get(context.Background(), key).Result()
		if err != nil {
			return "", err
		}
		return FormatJson(val)

	case "list":
		val, err := rds.rdsc.LRange(context.Background(), key, 0, -1).Result()
		if err != nil {
			return "", err
		}
		return FormatJson(val)

	case "hash":
		val, err := rds.rdsc.HGetAll(context.Background(), key).Result()
		if err != nil {
			return "", err
		}
		return FormatJson(val)

	case "set":
		val, err := rds.rdsc.SMembers(context.Background(), key).Result()
		if err != nil {
			return "", err
		}
		return FormatJson(val)

	case "zset":
		val, err := rds.rdsc.ZRangeWithScores(context.Background(), key, 0, -1).Result()
		if err != nil {
			return "", err
		}
		return FormatJson(val)

	case "stream":
		val, err := rds.rdsc.XRange(context.Background(), key, "-", "+").Result()
		if err != nil {
			return "", err
		}
		return FormatJson(val)

	case "none":
		return "(key does not exist)", nil

	default:
		return "sqltui has not implemented type: " + keyType, nil
	}
}

func FormatJson(a any) (string, error) {
	j, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return "", err
	}
	return string(j), nil
}
