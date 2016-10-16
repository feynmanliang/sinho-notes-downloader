package main

import (
	"fmt"
	"golang.org/x/net/html"
	"io"
	"net/http"
	"os"
	"regexp"
	"sync"
)

type crawlResult struct {
	className, title, gDocId string
}

var gDrivePrefixRE = regexp.MustCompile("https://drive.google.com/open\\?id=")
var slashRE = regexp.MustCompile("/")

func crawlLinks(url string) <-chan crawlResult {
	results := make(chan crawlResult)
	go func() {
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
		close(results)
	}()
	return results
}

func downloadCrawlResult(results <-chan crawlResult) <-chan string {
	messages := make(chan string)
	go func() {
		for result := range results {
			downloadUrl := fmt.Sprintf(
				"https://drive.google.com/uc?export=download&id=%s",
				result.gDocId)

			os.MkdirAll(fmt.Sprintf("downloads/%s", result.className), 0777)
			f, _ := os.Create(fmt.Sprintf("downloads/%s/%s.pdf", result.className, result.title))
			defer f.Close()

			resp, _ := http.Get(downloadUrl)
			defer resp.Body.Close()

			io.Copy(f, resp.Body)
			messages <- fmt.Sprintf("Downloaded %s %s", result.className, result.title)
		}
		close(messages)
	}()
	return messages
}

func downloadAllNotes(url string, numWorkers uint8) {
	results := crawlLinks(url)
	m := make([]<-chan string, numWorkers)
	for i := range m {
		m[i] = downloadCrawlResult(results)
	}
	for message := range merge(m...) {
		fmt.Println(message)
	}
}

func merge(cs ...<-chan string) <-chan string {
	var wg sync.WaitGroup
	out := make(chan string)

	// Start an output goroutine for each input channel in cs.  output
	// copies values from c to out until c is closed, then calls wg.Done.
	output := func(c <-chan string) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	// Start a goroutine to close out once all the output goroutines are
	// done.  This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func main() {
	url := "https://chewisinho.github.io"
	numWorkers := uint8(128)
	downloadAllNotes(url, numWorkers)
}
