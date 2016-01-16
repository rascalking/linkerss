package main

import (
	"bytes"
    "encoding/json"
	"flag"
    "fmt"
	"html/template"
	"log"
	"net/http"
	"os"
    "regexp"
    "strings"
	"time"

	"github.com/coreos/pkg/flagutil"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/gorilla/feeds"
    "golang.org/x/net/html"
	"golang.org/x/oauth2"
)


var textHtml = regexp.MustCompile("^text/html($|;)")


func main() {
	// parse flags
	flags := flag.NewFlagSet("linkerss", flag.ExitOnError)
	accessToken := flags.String("app-access-token", "",
		"Twitter application access token")
    listenAddress := flags.String("listen-address", "0.0.0.0:9999",
        "Address and port to listen on")
	flags.Parse(os.Args[1:])
	flagutil.SetFlagsFromEnv(flags, "TWITTER")
	if *accessToken == "" {
		log.Fatal("Application access token required")
	}

    // set up handlers
    http.HandleFunc("/user", func(w http.ResponseWriter, req *http.Request) {
        userHandler(accessToken, w, req)
    })

    // answer http requests
    log.Println("Listening on " + *listenAddress)
    log.Fatal(http.ListenAndServe(*listenAddress, nil))
}


func userHandler(accessToken *string, w http.ResponseWriter, req *http.Request) {
    log.Println("%v", req)
    screenName := req.URL.Query().Get("screenName")

	// set up the twitter client
	config := oauth2.Config{}
	token := oauth2.Token{AccessToken: *accessToken}
	httpClient := config.Client(oauth2.NoContext, &token)
	client := twitter.NewClient(httpClient)

	// fetch the timeline
	userTimelineParams := &twitter.UserTimelineParams{
		ScreenName: screenName, Count: 5}
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

func tweetsToFeed(tweets []twitter.Tweet, screenName string) *feeds.Feed {
	const htmlTemplate = `
    <div>
    <a href="{{.Link.Href}}">{{.Title}}</a> via {{.Author.Name}}
    </div>
    `
	templ := template.Must(template.New("item").Parse(htmlTemplate))

	feed := &feeds.Feed{
		Title:       fmt.Sprintf("@%s's linkerss", screenName),
		Link:        &feeds.Link{Href: fmt.Sprintf("https://twitter.com/%s",
                                                   screenName)},
		Description: fmt.Sprintf("Tweets with links in @%s's timeline",
                                 screenName),
		Author:      &feeds.Author{screenName, ""},
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
            // TODO: pdf, images?
            }

			// generate feed item
			item := &feeds.Item{
				Title:       title, // linked page title
				Link:        &feeds.Link{Href: u.ExpandedURL},
				Description: "", // "{LINK} via {USER} {TWEET}"
				Author:      author,
				//Created: t.CreatedAt, // TODO - parse string into time.Time
			}
            itemJson, _ := json.Marshal(item)
            var buf bytes.Buffer
            json.Indent(&buf, itemJson, "", "    ")
            buf.WriteTo(os.Stdout)

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
