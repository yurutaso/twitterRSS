package main

import (
	"fmt"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"log"
	"os"
	"strings"
	"time"
)

const (
	WOEID        int64  = 1118285 //Tokyo
	XML_TREND    string = `/var/www/html/trend.xml`
	XML_TIMELINE string = `/var/www/html/timeline.xml`
)

type RSS struct {
	description string
	href        string
	link        string
	title       string
	items       []*RSSItem
}

func (rss *RSS) String() string {
	items := ``
	for _, item := range rss.items {
		items += item.String()
	}
	return fmt.Sprintf(
		`<?xml version='1.0' encoding='UTF-8'?>
<rss xmlns:atom="http://www.w3.org/2005/Atom" version="2.0">
<channel>
<title>%s</title>
<link>%s</link>
<atom:link href="%s" rel="self" type="application/rss+xml"/>
<atom:link rel="hub" href="http://pubsubhubbub.appspot.com"/>
<description>%s</description>
<language></language>%s</channel>
</rss>`, rss.title, rss.link, rss.href, rss.description, items,
	)
}

type RSSItem struct {
	title       string
	link        string
	description string
	pubDate     string
	images      []*RSSImage
}

func replaceSpecialChars(old string) string {
	new := strings.Replace(old, `&`, `&amp;`, -1)
	new = strings.Replace(new, `<`, `&lt;`, -1)
	new = strings.Replace(new, `>`, `&gt;`, -1)
	return new
}
func (item *RSSItem) String() string {
	if item != nil {
		title := replaceSpecialChars(item.title)
		return fmt.Sprintf(`<item><title>%s</title><link>%s</link><description><![CDATA[%s%s]]></description><pubDate>%s</pubDate></item>`, title, item.link, ImagesString(item.images), item.description, item.pubDate)
	}
	return ``
}

type RSSImage struct {
	url   string
	title string
}

func ImagesString(images []*RSSImage) string {
	if images != nil {
		s := ""
		for _, image := range images {
			s += image.String()
		}
		return s
	}
	return ``
}

func (image *RSSImage) String() string {
	return fmt.Sprintf(`<img src="%s" alt="%s"></img>`, image.url, replaceSpecialChars(image.title))
}

func getTrendRSS(client *twitter.Client, woeid int64) RSS {
	trendlist, _, err := client.Trends.Place(woeid, nil)
	if err != nil {
		log.Fatal(err)
	}
	cnt := 1
	trends := trendlist[0].Trends
	loops := len(trends) / 10
	items := make([]*RSSItem, loops, loops)

	for i := 0; i < loops; i++ {
		content := ""
		for _, trend := range trends[i*10 : (i+1)*10] {
			content += fmt.Sprintf(`<p>%d. <a href="%s">%s</a></p>`, cnt, trend.URL, trend.Name)
			cnt += 1
		}
		t := time.Now()
		date := fmt.Sprintf("%d/%d/%d %d:%d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute())
		items = append(items, &RSSItem{
			title:       fmt.Sprintf(`Twitter Trend %s (%d-%d)`, date, i*10+1, (i+1)*10),
			link:        `https://twitter.com/` + date,
			description: content,
			pubDate:     t.String(),
			images:      nil,
		})
	}
	return RSS{
		title:       `Twitter Trend`,
		link:        `https://twitter.com`,
		description: `Twitter Trend`,
		items:       items,
		href:        os.Getenv(`TWITTER_DOMAIN`) + `trend`,
	}
}

func getTimelineRSS(client *twitter.Client, count int) RSS {
	tweets, _, err := client.Timelines.HomeTimeline(
		&twitter.HomeTimelineParams{
			Count:     count,
			TweetMode: `extended`,
		})
	if err != nil {
		log.Fatal(err)
	}

	items := make([]*RSSItem, count, count)
	for i, tweet := range tweets {
		uname := tweet.User.Name
		sname := tweet.User.ScreenName
		// Get images
		images := make([]*RSSImage, 0, 0)
		images = append(images, &RSSImage{
			url:   tweet.User.ProfileImageURL,
			title: sname,
		})
		// Read ExtendedEntities if exists.
		if tweet.ExtendedEntities != nil {
			if images2 := tweet.ExtendedEntities.Media; len(images2) > 0 {
				for _, image := range images2 {
					images = append(images, &RSSImage{
						url:   image.MediaURL,
						title: sname,
					})
				}
			}
		} else {
			// Read Entities only if ExtendedEntities does not exist.
			if images1 := tweet.Entities.Media; len(images1) > 0 {
				for _, image := range images1 {
					images = append(images, &RSSImage{
						url:   image.MediaURL,
						title: sname,
					})
				}
			}
		}
		items[i] = &RSSItem{
			title:       sname + `@` + uname,
			link:        fmt.Sprintf(`https://twitter.com/%s/status/%d`, sname, tweet.ID),
			description: tweet.FullText,
			pubDate:     tweet.CreatedAt,
			images:      images,
		}
	}
	return RSS{
		title:       `Twitter HomeTimeline`,
		link:        `https://twitter.com`,
		description: `Twitter HomeTimeline`,
		items:       items,
		href:        os.Getenv(`TWITTER_DOMAIN`) + `timeline`,
	}
}

func getClient() *twitter.Client {
	consumerKey := os.Getenv(`TWITTER_CONSUMER_KEY`)
	consumerSecret := os.Getenv(`TWITTER_CONSUMER_SECRET`)
	accessToken := os.Getenv(`TWITTER_ACCESS_TOKEN`)
	accessTokenSecret := os.Getenv(`TWITTER_ACCESS_TOKEN_SECRET`)
	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessTokenSecret)
	httpClient := config.Client(oauth1.NoContext, token)
	return twitter.NewClient(httpClient)
}

func main() {
	args := os.Args
	if len(args) != 2 {
		log.Fatal(fmt.Errorf(`Usage: ./twitterRSS [trend/timeline]`))
	}
	client := getClient()
	var file *os.File
	var rss RSS
	var err error
	switch args[1] {
	case `trend`:
		rss = getTrendRSS(client, WOEID)
		file, err = os.Create(XML_TREND)
	case `timeline`:
		rss = getTimelineRSS(client, 200)
		file, err = os.Create(XML_TIMELINE)
	default:
		log.Fatal(fmt.Errorf(`Usage: ./twitterRSS [trend/timeline]`))
	}
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	file.Write([]byte(rss.String()))
}
