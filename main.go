package main

import (
	"flag"
	"fmt"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"log"
	"os"
	"strings"
	"time"
)

var (
	optOutput = flag.String("o", "", "output")
	optTag    = flag.Bool("t", false, "print only <item></item> tag rather than the full xml")
)

const (
	WOEID int64  = 1118285 //Tokyo
	USAGE string = `Usage: ./twitterRSS [trend/timeline] or [search string]`
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
		items += item.String() + "\n"
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
<language></language>
%s
</channel>
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

func RSSItemFromTweet(tweet *twitter.Tweet) *RSSItem {
	uname := tweet.User.Name
	sname := tweet.User.ScreenName
	// Get a user profile image
	images := make([]*RSSImage, 0, 0)
	images = append(images, &RSSImage{
		url:   tweet.User.ProfileImageURL,
		title: sname,
	})
	// Read ExtendedEntities, if exist.
	if tweet.ExtendedEntities != nil {
		if _images := tweet.ExtendedEntities.Media; len(_images) > 0 {
			for _, image := range _images {
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
	return &RSSItem{
		//title:       sname + `@` + uname,
		title:       uname + `@` + sname,
		link:        fmt.Sprintf(`https://twitter.com/%s/status/%d`, sname, tweet.ID),
		description: tweet.FullText,
		pubDate:     tweet.CreatedAt,
		images:      images,
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
		items[i] = RSSItemFromTweet(&tweet)
	}
	return RSS{
		title:       `Twitter HomeTimeline`,
		link:        `https://twitter.com`,
		description: `Twitter HomeTimeline`,
		items:       items,
		href:        os.Getenv(`TWITTER_DOMAIN`) + `timeline`,
	}
}

func getSearchRSS(client *twitter.Client, s string, count int, opts []string) RSS {
	search, _, err := client.Search.Tweets(
		&twitter.SearchTweetParams{
			Count:     count,
			TweetMode: `extended`,
			Query:     s + ` ` + strings.Join(opts, ` `),
		})
	if err != nil {
		log.Fatal(err)
	}

	items := make([]*RSSItem, count, count)
	for i, tweet := range search.Statuses {
		items[i] = RSSItemFromTweet(&tweet)
	}
	return RSS{
		title:       fmt.Sprintf(`Twitter Search (%s)`, s),
		link:        `https://twitter.com`,
		description: fmt.Sprintf(`Twitter Search (%s)`, s),
		items:       items,
		href:        os.Getenv(`TWITTER_DOMAIN`) + `search/` + s,
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
	envs := []string{`TWITTER_CONSUMER_KEY`, `TWITTER_CONSUMER_SECRET`, `TWITTER_ACCESS_TOKEN`, `TWITTER_ACCESS_TOKEN_SECRET`}
	for _, key := range envs {
		if os.Getenv(key) == `` {
			log.Fatal(fmt.Errorf(`Error! Environmental variable named %s must be set to use REST API`, key))
		}
	}

	flag.Parse()
	//args := os.Args
	args := flag.Args()
	if len(args) < 1 {
		log.Fatal(fmt.Errorf(`Usage: ./twitterRSS [-o output] [trend, timeline, search string]`))
	}
	client := getClient()
	var rss RSS
	switch args[0] {
	case `trend`:
		if len(args) != 1 {
			log.Fatal(fmt.Errorf(USAGE))
		}
		rss = getTrendRSS(client, WOEID)
	case `timeline`:
		if len(args) != 1 {
			log.Fatal(fmt.Errorf(USAGE))
		}
		rss = getTimelineRSS(client, 200)
	case `search`:
		if len(args) < 2 {
			log.Fatal(fmt.Errorf(USAGE))
		}
		rss = getSearchRSS(client, args[1], 200, args[2:])
	default:
		log.Fatal(fmt.Errorf(USAGE))
	}

	var s string
	if *optTag {
		for _, item := range rss.items {
			s += item.String() + "\n"
		}
	} else {
		s = rss.String()
	}

	if len(*optOutput) == 0 {
		fmt.Println(s)
	} else {
		file, err := os.Create(*optOutput)
		if err != nil {
			log.Fatal(err)
		}
		_, err = file.WriteString(s)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
	}
}
