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

	// Create a pending query map if it doesn't exist
	_, err := wrap.RedisClient.Exists("queries:pending").Result()
	if err != nil {
		panic(err)
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
	err = wrap.RedisClient.Set("uuid:"+host.UUID, nodeKey, 0).Err()
	if err != nil {
		fmt.Printf("Redis Error: %s\n", err)
		return err
	}
	return nil
}

func (wrap *GoServerDatabaseWrapper) ScheduleQuery(uuid, query string) (string, error) {
	nodeKey, err := wrap.RedisClient.Get("uuid:" + uuid).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("No such host")
	} else if err != nil {
		return "", err
	}
	// Generate queryName, random strings work well and don't leak information
	// or intention (though of course the query itself will be visible to a
	// watching attacker)
	queryName := randomString(32) // 192 bits of entropy with current function

	// Add new query to host
	err = wrap.RedisClient.HSet("queries:nodekey:"+nodeKey, queryName, query).Err()
	if err != nil {
		// Someone messing with the DB? Query map is gone...
		fmt.Printf("Redis Error: %s\n", err)
		return "", err
	}

	// Mark the query as pending while waiting for it to come back
	err = wrap.RedisClient.HSet("queries:pending", queryName, query).Err()
	if err != nil {
		return "", err
	}

	// Add it to the pending set for the host
	err = wrap.RedisClient.HSet("queries:pending:nodekey:"+nodeKey, queryName, query).Err()
	if err != nil {
		return "", err
	}
	return queryName, nil
}

func (wrap *GoServerDatabaseWrapper) GetHostInfo(uuid string) (string, Host, error) {
	// First we need to find the nodeKey of the UUID in question
	nodeKey, err := wrap.RedisClient.Get("uuid:" + uuid).Result()
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
	return nodeKey, Host{
			UUID:         uuid,
			ComputerName: hostData[0].(string),
			Version:      hostData[1].(string),
			Platform:     hostData[2].(string)},
		nil
}

func (wrap *GoServerDatabaseWrapper) NodeKeyEnrolled(nodeKey string) (bool, error) {
	exists, err := wrap.RedisClient.Exists("nodekey:" + nodeKey).Result()
	if err != nil {
		return false, err
	}
	if exists > 0 {
		return true, nil
	} else {
		return false, nil
	}
}

//TODO Add query run status into the database and return here
func (wrap *GoServerDatabaseWrapper) FetchResults(queryName string) (string, string, error) {
	res, err := wrap.RedisClient.HGet("queries:complete", queryName).Result()
	if err == redis.Nil {
		res, err = wrap.RedisClient.HGet("queries:pending", queryName).Result()
		if err == nil {
			return "[]", "Pending", nil
		}
		if err == redis.Nil {
			return "[]", "Unknown", nil
		}
		return "", "", err
	} else if err != nil {
		// Someone messing with the DB? Query map is gone...
		fmt.Printf("Redis Error: %s\n", err)
		return "", "", err
	}

	return res, "Complete", nil
}

func (wrap *GoServerDatabaseWrapper) GetHostQueries(nodeKey string) (map[string]string, error) {
	pendingQueries, err := wrap.RedisClient.HGetAll("queries:nodekey:" + nodeKey).Result()
	if err == redis.Nil {
		return map[string]string{}, fmt.Errorf("No such host")
	} else if err != nil {
		return map[string]string{}, err
	}

	return pendingQueries, nil
}

func (wrap *GoServerDatabaseWrapper) GetPendingHostQueries(nodeKey string) (map[string]string, error) {
	pendingQueries, err := wrap.RedisClient.HGetAll("queries:pending:nodekey:" + nodeKey).Result()
	if err == redis.Nil {
		return map[string]string{}, fmt.Errorf("No such host")
	} else if err != nil {
		return map[string]string{}, err
	}

	return pendingQueries, nil
}

func (wrap *GoServerDatabaseWrapper) PutPendingQueryResults(queryName, results, nodeKey string) {
	// First store the results
	err := wrap.RedisClient.HSet("queries:complete", queryName, results).Err()
	if err != nil {
		fmt.Printf("Redis Error: %s\n", err)
		return
	}
	// Mark the query as complete by removing it from the global and host
	// pending sets
	err = wrap.RedisClient.HDel("queries:pending", queryName).Err()
	if err != nil {
		fmt.Printf("Error Occured Marking %s as complete: %s\n", queryName, err)
	}
	err = wrap.RedisClient.HDel("queries:pending:nodekey:"+nodeKey, queryName).Err()
	if err != nil {
		fmt.Printf("Error Occured Marking %s as complete: %s\n", queryName, err)
	}
}
