package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/pkg/flagutil"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/gorilla/feeds"
	"golang.org/x/net/html"
	"golang.org/x/oauth2"
)

var textHtml = regexp.MustCompile("^text/html($|;)")
var (
	flags = flag.NewFlagSet("linkerss", flag.ExitOnError)
	accessToken = flags.String("app-access-token", "",
		"Twitter application access token")
	listenAddress = flags.String("listen-address", "0.0.0.0:9999",
		"Address and port to listen on")
	maxTweets = flags.Int("max-tweets", 100,
		"Max number of tweets to pull from Twitter")
	defaultNumTweets = flags.Int("num-tweets", 20,
		"Default number of tweets to pull from twitter, can be overridden" +
		"via query parameter")
)

func main() {
	// parse flags
	flags.Parse(os.Args[1:])
	flagutil.SetFlagsFromEnv(flags, "TWITTER")
	if *accessToken == "" {
		log.Fatal("Application access token required")
	}
	if *defaultNumTweets > *maxTweets {
		log.Fatal("--num-tweets cannot be larger than --max-tweets")
	}

	// set up handlers
	http.HandleFunc("/user", func(w http.ResponseWriter, req *http.Request) {
		userHandler(accessToken, w, req)
	})

	// answer http requests
	log.Println("Listening on " + *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

// TODO: caching somewhere
func userHandler(accessToken *string, w http.ResponseWriter, req *http.Request) {
	log.Printf("%+v\n", req)

	// TODO: has to be a cleaner pattern than this to get an int from query params
	numTweetsStr := req.URL.Query().Get("numTweets")
	numTweets := *defaultNumTweets
	if numTweetsStr != "" {
		var err error
		numTweets, err = strconv.Atoi(numTweetsStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			// TODO: include explanation in body
			return
		}
	}

	// set up the twitter client
	config := oauth2.Config{}
	token := oauth2.Token{AccessToken: *accessToken}
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

// TODO: split this out into a separate package
func tweetsToFeed(tweets []twitter.Tweet, screenName string) *feeds.Feed {
	const htmlTemplate = `
    <div>
      <a href="{{.Link.Href}}">{{.Title}}</a>
    </div>
    <div>
	via <a href="https://twitter.com/{{.Author.Name}}">{{.Author.Name}}</a>
    </div>
    `
	templ := template.Must(template.New("item").Parse(htmlTemplate))

	feed := &feeds.Feed{
		Title: fmt.Sprintf("@%s's linkerss", screenName),
		Link: &feeds.Link{Href: fmt.Sprintf("https://twitter.com/%s",
			screenName)},
		Description: fmt.Sprintf("Links from tweets in @%s's timeline",
			screenName),
		Author:  &feeds.Author{screenName, ""},
		Created: time.Now(),
	}

	// TODO: parallelize item generation, but retain original order
	for _, t := range tweets {
		author := &feeds.Author{t.User.Name, ""}
		if t.RetweetedStatus != nil {
			author = &feeds.Author{t.RetweetedStatus.User.Name, ""}
		}


		// TODO: head the url, then split handling out by content-type
		for _, u := range t.Entities.Urls {
			// fetch page
			resp, err := http.Get(u.ExpandedURL)
			if err != nil {
				log.Printf("unable to retrieve %s: %s", u.ExpandedURL, err)
				continue
			}
			defer resp.Body.Close()

			// figure out a title
			title := u.ExpandedURL
			switch {
			case textHtml.MatchString(resp.Header.Get("Content-Type")):
				doc, err := html.Parse(resp.Body)
				if err != nil {
					log.Printf("unable to parse the body of %s: %s",
						u.ExpandedURL, err)
					continue
				}
				title = getTitle(doc)
			}

			// parse out the timestamp
			created, err := time.Parse(time.RubyDate, t.CreatedAt)
			if err != nil {
				log.Printf("unable to parse tweet's created_at value %s: %s",
					t.CreatedAt, err)
			}

			// generate feed item
			item := &feeds.Item{
				Title: title, // linked page title
				Link: &feeds.Link{Href: u.ExpandedURL},
				Description: "", // "{LINK} via {USER} {TWEET}"
				Author: author,
				Created: created,
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

func getTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		return strings.TrimSpace(n.FirstChild.Data)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		title := getTitle(c)
		if title != "" {
			return title
		}
	}
	return ""
}
