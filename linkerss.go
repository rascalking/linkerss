package main

import (
	"bytes"
	"flag"
	"log"
	//"io/ioutil"
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/coreos/pkg/flagutil"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/gorilla/feeds"
	//"golang.org/x/net/html"
	"golang.org/x/oauth2"
)

// TODO - don't hardcode @rascalking
// TODO - put it behind an http server

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

	// filter down to just tweets with urls
	urlTweets := make([]twitter.Tweet, 0)
	for _, t := range tweets {
		if t.Entities.Urls != nil {
			urlTweets = append(urlTweets, t)
		}
	}

	// turn them into feed entries
	feed := tweetsToFeed(urlTweets)

	// output as rss
	err = feed.WriteRss(os.Stdout)
	if err != nil {
		log.Fatal("error outputting as rss: %v", err)
	}
}

func tweetsToFeed(tweets []twitter.Tweet) *feeds.Feed {
	const htmlTemplate = `
    <div>
    <a href="{{.Link}}">{{.Title}}</a> via {{.Author.Name}}
    </div>
    `
	templ := template.Must(template.New("item").Parse(htmlTemplate))

	feed := &feeds.Feed{
		Title:       "@rascalking's linkerss",
		Link:        &feeds.Link{Href: "https://twitter.com/rascalking"},
		Description: "Tweets with links in @rascalking's timeline",
		Author:      &feeds.Author{"David Bonner", "dbonner@gmail.com"},
		Created:     time.Now(),
	}

	for _, t := range tweets {
		// TODO - look at retweeted_status to find RTer, not original author
		author := &feeds.Author{t.User.Name, ""}

		for _, u := range t.Entities.Urls {
			// fetch page
			resp, err := http.Get(u.ExpandedURL)
			if err != nil {
				log.Printf("unable to retrieve %s: %s", u.ExpandedURL, err)
				continue
			}
			defer resp.Body.Close()
			/*
			   body, err := ioutil.ReadAll(resp.Body)
			   if err != nil {
			       log.Printf("unable to read the body of %s: %s",
			                   u.ExpandedURL, err)
			       continue
			   }
			*/

			// figure out a title
			title := u.ExpandedURL
			/*
			   if resp.Header.Get("Content-Type") == "text/html" {
			       doc, err := html.Parse(resp.Body)
			       if err != nil {
			           log.Printf("unable to parse the body of %s: %s",
			                       u.ExpandedURL, err)
			       }
			   }
			*/

			// generate feed item
			item := &feeds.Item{
				Title:       title, // linked page title
				Link:        &feeds.Link{Href: u.ExpandedURL},
				Description: "", // "{LINK} via {USER} {TWEET}"
				Author:      author,
				//Created: t.CreatedAt, // TODO - parse string into time.Time
			}

			// now build the item description
			buffer := new(bytes.Buffer)
			templ.Execute(buffer, item)
			item.Description = buffer.String()

			// and add it in
			feed.Items = append(feed.Items, item)
		}
	}

	return feed
}
