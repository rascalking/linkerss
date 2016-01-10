package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/coreos/pkg/flagutil"
	"github.com/dghubble/go-twitter/twitter"
	"golang.org/x/oauth2"
)

func main() {
	// grab the acccess token
	flags := flag.NewFlagSet("linkerss", flag.ExitOnError)
	accessToken := flags.String("app-access-token", "",
		"Twitter Application Access Token")
	flags.Parse(os.Args[1:])
	flagutil.SetFlagsFromEnv(flags, "TWITTER")
	if *accessToken == "" {
		log.Fatal("Application Access Token required")
	}

	// set up the twitter client
	config := &oauth2.Config{}
	token := &oauth2.Token{AccessToken: *accessToken}
	httpClient := config.Client(oauth2.NoContext, token)
	client := twitter.NewClient(httpClient)

	// fetch the timeline
	userTimelineParams := &twitter.UserTimelineParams{
		ScreenName: "rascalking", Count: 5}
	tweets, _, err := client.Timelines.UserTimeline(userTimelineParams)
    if err != nil {
        log.Fatal("error getting user timeline: %v", err)
    }
	fmt.Printf("USER TIMELINE:\n%+v\n", tweets)
}
