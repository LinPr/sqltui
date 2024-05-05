package redis

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

var RdsClinet *RDS

type RDS struct {
	rdsc *redis.Client
}

func NewRDS(rdsOpt *redis.Options) (*RDS, error) {
	if RdsClinet != nil {
		return RdsClinet, nil
	}

	// rdb := redis.NewClient(&redis.Options{
	// 	Addr:     "localhost:6379",
	// 	Username: "",
	// 	Password: "", // no password set
	// 	DB:       0,  // use default DB
	// })

	rdsc := redis.NewClient(rdsOpt)

	if _, err := rdsc.Ping(context.Background()).Result(); err != nil {
		return nil, err
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
	log.Println("")
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
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (rds *RDS) GetValue(key string) (string, error) {
	keyType := rds.rdsc.Type(context.Background(), key).Val()
	log.Printf("keyType: %s", keyType)
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
		log.Printf("zset: %+v", val)
		return FormatJson(val)

	case "bitmap":
		val, err := rds.rdsc.GetBit(context.Background(), key, 0).Result()
		if err != nil {
			return "", err
		}
		return FormatJson(val)

	default:
		return "sqltui has not implimented type" + keyType, nil
	}
}

func FormatJson(a any) (string, error) {
	j, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return "", err
	}
	return string(j), nil
}
