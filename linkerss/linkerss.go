package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/coreos/pkg/flagutil"
	"github.com/garyburd/redigo/redis"
	"github.com/rascalking/linkerss"
)

var (
	flags = flag.NewFlagSet("linkerss", flag.ExitOnError)
	accessToken = flags.String("app-access-token", "",
		"Twitter application access token")
	listenAddress = flags.String("listen-address", "0.0.0.0:9999",
		"Address and port to listen on")
	maxNumTweets = flags.Int("max-num-tweets", 100,
		"Max number of tweets to pull from Twitter")
	defaultNumTweets = flags.Int("num-tweets", 20,
		"Default number of tweets to pull from twitter, can be overridden " +
		"via query parameter")
	redisServer = flags.String("redis-server", ":6379", "Redis server")
	redisPassword = flags.String("redis-password", "", "Redis password")
)


func main() {
	// parse flags
	flags.Parse(os.Args[1:])
	flagutil.SetFlagsFromEnv(flags, "LINKERSS")
	if *accessToken == "" {
		log.Fatal("Application access token required")
	}
	if *defaultNumTweets > *maxNumTweets {
		log.Fatal("--num-tweets cannot be larger than --max-num-tweets")
	}

	// set up the redis pool
	pool := newPool(*redisServer, *redisPassword)

	// set up handlers
	linkerssHandler := linkerss.LinkerssHandler{*accessToken, *defaultNumTweets,
						*maxNumTweets, pool}
	http.Handle("/user", linkerssHandler)

	// log our args before we start listening
	log.Println("Listening on:" + *listenAddress)
	log.Println("Default number of tweets:", *defaultNumTweets)
	log.Println("Maximum number of tweets:", *maxNumTweets)

	// answer http requests
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}


func newPool(server, password string) *redis.Pool {
	return &redis.Pool{
		MaxIdle: 3,
		IdleTimeout: 240 * time.Second,
		Dial: func () (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}
