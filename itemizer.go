package linkerss

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/feeds"
	"golang.org/x/net/html"
)

var tokens = make(chan struct{}, 20)
var textHtml = regexp.MustCompile("^text/html($|;)")
const defaultDescription = `
<div>
  <a href="{{.Link.Href}}">{{.Title}}</a>
</div>
<div>
via <a href="https://twitter.com/{{.Author.Name}}">{{.Author.Name}}</a>
</div>
`
var defaultDescriptionTemplate = template.Must(template.New("item").Parse(defaultDescription))

type itemizerResult struct {
	position int
	item *feeds.Item
}


func tweetsToFeed(tweets []twitter.Tweet, screenName string, pool *redis.Pool) *feeds.Feed {
	// hand out work
	position := 0
	resultChan := make(chan itemizerResult)
	for _, tweet := range tweets {
		for _, url := range tweet.Entities.Urls {
			log.Printf("Launching itemize(%d, %s, %d) in a goroutine",
						position, url.ExpandedURL, tweet.ID, resultChan)
			go itemize(position, url.ExpandedURL, &tweet, resultChan, pool)
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


func itemize(position int, url string, tweet *twitter.Tweet, resultChan chan<- itemizerResult, pool *redis.Pool) {
	log.Printf("itemize for position %d acquiring token", position)
	tokens <- struct{}{}  // acquire a token
	log.Printf("itemize for position %d acquired token", position)
	// TODO: defer releasing the token, optionally sending result

	// start with a vanilla item
	item := getDefaultItem(url, tweet)

	// inspect the linked entity for better details
	body, contentType, err := httpGet(url, pool)
	if err != nil {
		log.Printf("unable to retrieve %s: %s", url, err)
	} else {
		switch contentType {
		case "text/html":
			augmentItemHTML(item, &body)
		}
	}

	// generate the description again now that we have a real title
	item.Description = applyItemTemplate(item, defaultDescriptionTemplate)

	log.Printf("itemize for position %d returning result", position)
	<-tokens  // release the token
	resultChan <- itemizerResult{position: position, item: item}
}


func getDefaultItem(url string, tweet *twitter.Tweet) *feeds.Item {
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

	item := &feeds.Item{
		Title: url,
		Link: &feeds.Link{Href: url},
		Description: "",
		Author: author,
		Created: created,
	}

	item.Description = applyItemTemplate(item, defaultDescriptionTemplate)

	return item
}


func applyItemTemplate(item *feeds.Item, templ *template.Template) string {
	buffer := new(bytes.Buffer)
	templ.Execute(buffer, item)
	return buffer.String()
}


func augmentItemHTML(item *feeds.Item, body *string) {
	doc, err := html.Parse(strings.NewReader(*body))
	if err != nil {
		// TODO: pass url in for error logging?  return err?
		log.Printf("unable to parse the body: %s", err)
	}
	item.Title = getHTMLTitle(doc)
	// TODO - snippet of html body in description
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


func httpGet(url string, pool *redis.Pool) (body, contentType string, err error) {
	// TODO: extend errors we encounter rather than logging and returning
	conn := pool.Get()
	defer conn.Close()

	// check to see if we have it cached
	cached, err := redis.Bool(conn.Do("EXISTS", url))
	if err != nil {
		log.Printf("error looking for %s in redis: %s", url, err)
		cached = false
	}

	if cached {
		// if we hit an error with redis, fall back to just getting it again
		reply, err := redis.Values(conn.Do("HMGET", url, "body", "contentType"))
		if err == nil {
			if _, err := redis.Scan(reply, &body, &contentType); err != nil {
				log.Printf("error scanning redis result from %s: %s", url, err)
			}
		} else {
			log.Printf("error retrieving %s from redis: %s", url, err)
		}
	}

	if body == "" && contentType == "" {
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("unable to retrieve %s: %s", url, err)
			return "", "", err
		}
		defer resp.Body.Close()

		// read the body
		contentType = strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0]
		contents, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("error reading response body from %s: %s", url, err)
			return "", "", err
		}
		body = string(contents)

		// cache it
		var args []interface{}
		args = append(args, url, "body", body, "contentType", contentType)
		if _, err = conn.Do("HMSET", args...); err != nil {
			log.Printf("error storing %s in redis: %s", url, err)
		}
	}

	return body, contentType, nil
}
