package linkerss

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/gorilla/feeds"
	"golang.org/x/net/html"
)

var tokens = make(chan struct{}, 20)
var textHtml = regexp.MustCompile("^text/html($|;)")

type itemizerResult struct {
	position int
	item *feeds.Item
}

func tweetsToFeed(tweets []twitter.Tweet, screenName string) *feeds.Feed {
	// hand out work
	position := 0
	resultChan := make(chan itemizerResult)
	for _, tweet := range tweets {
		for _, url := range tweet.Entities.Urls {
			log.Printf("Launching itemize(%d, %s, %d) in a goroutine",
						position, url.ExpandedURL, tweet.ID, resultChan)
			go itemize(position, url.ExpandedURL, tweet, resultChan)
			position++
		}
	}
	log.Printf("Launched %d goroutines", position)

	// collect results
	items := make([]*feeds.Item, position)
	for i := 0; i < position; i++ {
		result, ok := <-resultChan
		log.Printf("Collected %v, %v from the result channel",
					result, ok)
		if (!ok) {
			// TODO - how to handle someone else closing this channel?
			log.Printf("unexpected close of the result channel")
			return nil
		}
		items[result.position] = result.item
	}
	log.Printf("Finished collecting %d items", position)

	// construct the feed
	feed := &feeds.Feed{
		Title: fmt.Sprintf("@%s's linkerss", screenName),
		Link: &feeds.Link{Href: fmt.Sprintf("https://twitter.com/%s",
			screenName)},
		Description: fmt.Sprintf("Links from tweets in @%s's timeline",
			screenName),
		Author:  &feeds.Author{screenName, ""},
		Created: time.Now(),
		Items: items,
	}

	log.Printf("Returning feed")
	return feed
}

func itemize(position int, url string, tweet twitter.Tweet, resultChan chan<- itemizerResult) {
	log.Printf("itemize for position %d acquiring token", position)
	tokens <- struct{}{}  // acquire a token
	log.Printf("itemize for position %d acquired token", position)

	// figure out the fallback item values
	author := &feeds.Author{tweet.User.Name, ""}
	if tweet.RetweetedStatus != nil {
		author = &feeds.Author{tweet.RetweetedStatus.User.Name, ""}
	}
	created, err := time.Parse(time.RubyDate, tweet.CreatedAt)
	if err != nil {
		log.Printf("unable to parse tweet's created_at value %s: %s",
			tweet.CreatedAt, err)
		created = time.Now()
	}

	// generate feed item
	item := &feeds.Item{
		Title: url,
		Link: &feeds.Link{Href: url},
		Description: "",
		Author: author,
		Created: created,
	}

	// get the html title if possible
	log.Printf("itemize for position %d retrieving %s", position, url)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("unable to retrieve %s: %s", url, err)
	} else {
		defer resp.Body.Close()
		switch {
		case textHtml.MatchString(resp.Header.Get("Content-Type")):
			doc, err := html.Parse(resp.Body)
			if err != nil {
				log.Printf("unable to parse the body of %s: %s",
					url, err)
			}
			item.Title = getHTMLTitle(doc)
		}
	}

	// flesh out the description given the item contents
	const htmlTemplate = `
    <div>
      <a href="{{.Link.Href}}">{{.Title}}</a>
    </div>
    <div>
	via <a href="https://twitter.com/{{.Author.Name}}">{{.Author.Name}}</a>
    </div>
    `
	templ := template.Must(template.New("item").Parse(htmlTemplate))
	buffer := new(bytes.Buffer)
	templ.Execute(buffer, item)
	item.Description = buffer.String()

	log.Printf("itemize for position %d returning result", position)
	<-tokens  // release the token
	resultChan <- itemizerResult{position: position, item: item}
}

func getHTMLTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		if n.FirstChild == nil {
			log.Printf("node %v seems to be a title, has no children", n)
			return ""
		}
		return strings.TrimSpace(n.FirstChild.Data)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		title := getHTMLTitle(c)
		if title != "" {
			return title
		}
	}
	return ""
}
