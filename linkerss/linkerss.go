package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/coreos/pkg/flagutil"
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

	// set up handlers
	linkerssHandler := linkerss.LinkerssHandler{*accessToken, *defaultNumTweets,
						*maxNumTweets}
	http.Handle("/user", linkerssHandler)

	// answer http requests
	log.Println("Listening on " + *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
