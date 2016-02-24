package linkerss

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/dghubble/go-twitter/twitter"
	"golang.org/x/oauth2"
)

type LinkerssHandler struct {
	AccessToken string
	DefaultNumTweets int
	MaxNumTweets int
}

// TODO: caching somewhere
// TODO: implement OPTIONS
func (l LinkerssHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Printf("%+v\n", req)

	// TODO: has to be a cleaner pattern than this to get an int from query params
	numTweets := l.DefaultNumTweets
	numTweetsStr := req.URL.Query().Get("numTweets")
	if numTweetsStr != "" {
		var err error
		numTweets, err = strconv.Atoi(numTweetsStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprintf("invalid numTweets %s", numTweetsStr)))
			return
		}
	}
	if numTweets > l.MaxNumTweets {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("invalid numTweets %d, must be less than %d",
									numTweets, l.MaxNumTweets)))
		return
	}

	// set up the twitter client
	config := oauth2.Config{}
	token := oauth2.Token{AccessToken: l.AccessToken}
	httpClient := config.Client(oauth2.NoContext, &token)
	client := twitter.NewClient(httpClient)

	// fetch the timeline
	screenName := req.URL.Query().Get("screenName")
	userTimelineParams := &twitter.UserTimelineParams{
		ScreenName: screenName, Count: numTweets}
	tweets, _, err := client.Timelines.UserTimeline(userTimelineParams)
	if err != nil {
		log.Fatal("error getting user timeline: %v", err)
	}

	// filter down to just tweets with urls
	urlTweets := make([]twitter.Tweet, 0)
	for _, t := range tweets {
		if t.Entities.Urls != nil {
			urlTweets = append(urlTweets, t)
		}
	}

	// turn them into feed entries
	feed := tweetsToFeed(urlTweets, screenName)

	// write back to client as rss
	err = feed.WriteRss(w)
	if err != nil {
		log.Fatal("error outputting as rss: %v", err)
	}
}
