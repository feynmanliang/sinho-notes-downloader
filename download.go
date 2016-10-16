package main

import (
	"fmt"
	"golang.org/x/net/html"
	"io"
	"net/http"
	"os"
	"regexp"
)

type crawlResult struct {
	className, title, gDocId string
}

var gDrivePrefixRE = regexp.MustCompile("https://drive.google.com/open\\?id=")
var slashRE = regexp.MustCompile("/")

func crawlLinks(url string, results chan<- crawlResult, jobFinished chan<- bool) {
	resp, _ := http.Get(url)
	defer resp.Body.Close()

	z := html.NewTokenizer(resp.Body)
	var className string
	for {
		tt := z.Next()

		switch tt {
		case html.ErrorToken:
			// End of the document, we're done
			close(results)
			return
		case html.StartTagToken:
			t := z.Token()
			switch t.Data {
			case "h3":
				z.Next()
				className = slashRE.ReplaceAllString(z.Token().Data, "-")
			case "a":
				for _, a := range t.Attr {
					if gDrivePrefixRE.MatchString(a.Val) {
						url := a.Val
						gDocId := gDrivePrefixRE.ReplaceAllString(url, "")

						z.Next()
						title := z.Token().Data

						results <- crawlResult{className, title, gDocId}
					}
				}
			}
		}
	}
}

func downloadCrawlResult(result crawlResult, jobFinished chan<- bool) {
	downloadUrl := fmt.Sprintf(
		"https://drive.google.com/uc?export=download&id=%s",
		result.gDocId)

	os.Mkdir(fmt.Sprintf("%s", result.className), 0777)
	f, _ := os.Create(fmt.Sprintf("downloads/%s/%s.pdf", result.className, result.title))
	defer f.Close()

	resp, _ := http.Get(downloadUrl)
	defer resp.Body.Close()

	io.Copy(f, resp.Body)
	jobFinished <- true
}

func downloadAllNotes(url string) {
	results := make(chan crawlResult, 8)
	jobFinished := make(chan bool)
	go crawlLinks(url, results, jobFinished)

	numJobs := 0
	for result := range results {
		numJobs++
		fmt.Printf("Downloading %s %s\n", result.className, result.title)
		go downloadCrawlResult(result, jobFinished)
	}
	for numJobs > 0 {
		select {
		case <-jobFinished:
			numJobs--
		}
	}
}

func main() {
	url := "https://chewisinho.github.io"
	downloadAllNotes(url)
}
