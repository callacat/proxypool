package main

import (
	"fmt"
	"github.com/gocolly/colly/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"net/url"
	"slices"
	"strings"
)

// 从https://www.freefq.com/获取分享链接的插件
func main() {
	link := "https://www.freefq.com/"
	c := colly.NewCollector()
	c2 := colly.NewCollector()
	c3 := colly.NewCollector()
	shareLinks := make([]string, 0)
	// Find and visit all links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		if e.Attr("title") != "" {
			t, _ := simplifiedchinese.GBK.NewDecoder().Bytes([]byte(e.Attr("title")))
			title := string(t)
			if strings.Contains(title, "免费账号分享") || strings.Contains(title, "免费节点分享") {
				to, _ := url.JoinPath(link, e.Attr("href"))
				c2.Visit(to)

			}
		}

	})

	c2.OnHTML("a[href]", func(e *colly.HTMLElement) {
		if strings.Contains(e.Attr("title"), ".htm") {
			c3.Visit(e.Attr("href"))
		}
	})

	c3.OnHTML("p", func(e *colly.HTMLElement) {
		ctn := strings.TrimSpace(e.Text)
		for _, s := range strings.Fields(ctn) {
			s = strings.TrimSpace(s)
			protocol := strings.Split(s, "://")
			protocols := []string{"ss", "ssr", "vmess", "vless", "trojan"}
			if include := slices.Contains(protocols, protocol[0]); include && len(protocol) > 0 {
				shareLinks = append(shareLinks, s)
			}
		}
	})
	c.Visit(link)
	fmt.Print(strings.Join(shareLinks, "\n"))
}
