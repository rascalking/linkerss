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

var textHtml = regexp.MustCompile("^text/html($|;)")

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
