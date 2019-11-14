package main

import (
	"fmt"

	"github.com/go-redis/redis/v7"
)

type GoServerDatabaseWrapper struct {
	RedisClient *redis.Client
}

func (wrap *GoServerDatabaseWrapper) Init() {
	wrap.RedisClient = redis.NewClient(&redis.Options{
		Addr:     "goserver-redis:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	_, err := wrap.RedisClient.Ping().Result()
	if err != nil {
		fmt.Printf("Redis Error Occured: %s\n", err)
		panic(err)
	} else {
		fmt.Printf("[DBWrapper] Redis is up\n")
	}
}

func (wrap *GoServerDatabaseWrapper) EnrollNewHost(nodeKey string, host Host) error {
	redisMap := make(map[string]interface{}, 0)
	redisMap["UUID"] = host.UUID
	redisMap["ComputerName"] = host.ComputerName
	redisMap["OsqueryVersion"] = host.Version
	redisMap["Platform"] = host.Platform

	// Register Host Data For NodeKey Lookups
	err := wrap.RedisClient.HMSet("nodekey:"+nodeKey, redisMap).Err()
	if err != nil {
		fmt.Printf("Redis Error: %s\n", err)
		return err
	}
	// Register UUID to NodeKey Mapping
	err = wrap.RedisClient.Set("host:"+host.UUID, nodeKey, 0).Err()
	if err != nil {
		fmt.Printf("Redis Error: %s\n", err)
		return err
	}
	return nil
}

func (wrap *GoServerDatabaseWrapper) ScheduleNewQuery(uuid, query string) (string, error) {
	return "", nil
}

func (wrap *GoServerDatabaseWrapper) GetHostInfo(uuid string) (string, Host, error) {
	// First we need to find the nodeKey of the UUID in question
	nodeKey, err := wrap.RedisClient.Get("host:"+uuid).Result()
	if err == redis.Nil {
		return "", Host{}, fmt.Errorf("No such host")
	} else if err != nil {
		return "", Host{}, err
	}

	// Now we need to reconstruct the host struct from Redis Data
	hostData, err := wrap.RedisClient.HMGet("nodekey:"+nodeKey, "ComputerName", "OsqueryVersion", "Platform").Result()
	if err == redis.Nil {
		return "", Host{}, fmt.Errorf("No host information. Need to force re-enrollment")
	} else if err != nil {
		return "", Host{}, err
	}
	return nodeKey, Host {
						UUID:uuid,
						ComputerName:hostData[0].(string),
						Version:hostData[1].(string),
						Platform:hostData[2].(string)},
			nil
}

func (wrap *GoServerDatabaseWrapper) NodeKeyEnrolled(nodeKey string) (bool, error) {
	exists, err := wrap.RedisClient.Exists("nodekey:"+nodeKey).Result()
	if err != nil {
		return false, err
	}
	if exists > 0 {
		return true, nil
	} else {
		return false, nil
	}
}

func (wrap *GoServerDatabaseWrapper) FetchResults(queryName string) (string, error) {
	return "", nil
}
