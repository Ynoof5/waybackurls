package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {

	var domains []string

	var days int
	flag.IntVar(&days, "days", 0, "number of days back to fetch URLs for (1-30)")

	var noSubs bool
	flag.BoolVar(&noSubs, "no-subs", false, "don't include subdomains of the target domain")

	var getVersionsFlag bool
	flag.BoolVar(&getVersionsFlag, "get-versions", false, "list URLs for crawled versions of input URL(s)")

	flag.Parse()

	if flag.NArg() > 0 {
		// fetch for a single domain
		domains = []string{flag.Arg(0)}
	} else {

		// fetch for all domains from stdin
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			domains = append(domains, sc.Text())
		}

		if err := sc.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
		}
	}

	// get-versions mode
	if getVersionsFlag {

		for _, u := range domains {
			versions, err := getVersions(u)
			if err != nil {
				continue
			}
			fmt.Println(strings.Join(versions, "\n"))
		}

		return
	}

	var fetchFns []fetchFn

	if days > 0 {
		// Get URLs for specified number of days back
		fetchFns = append(fetchFns, getWaybackURLs)
	} else {
		// Get all URLs without setting any date restriction
		fetchFns = append(fetchFns, getWaybackURLsAll)
	}

	for _, domain := range domains {

		var wg sync.WaitGroup
		wurls := make(chan wurl)

		for _, fn := range fetchFns {
			wg.Add(1)
			fetch := fn
			go func() {
				defer wg.Done()
				resp, err := fetch(domain, noSubs, days)
				if err != nil {
					return
				}
				for _, r := range resp {
					if noSubs && isSubdomain(r.url, domain) {
						continue
					}
					wurls <- r
				}
			}()
		}

		go func() {
			wg.Wait()
			close(wurls)
		}()

		seen := make(map[string]bool)
		for w := range wurls {
			if _, ok := seen[w.url]; ok {
				continue
			}
			seen[w.url] = true

			fmt.Println(w.url)
		}
	}

}

type wurl struct {
	date string
	url  string
}

type fetchFn func(string, bool, int) ([]wurl, error)

func getWaybackURLs(domain string, noSubs bool, days int) ([]wurl, error) {
	subsWildcard := "*."
	if noSubs {
		subsWildcard = ""
	}

	fromDate := time.Now().AddDate(0, 0, -days).Format("20060102")
	toDate := time.Now().Format("20060102")

	res, err := http.Get(
		fmt.Sprintf("http://web.archive.org/cdx/search/cdx?url=%s%s/*&output=json&collapse=urlkey&from=%s", subsWildcard, domain, fromDate),
	)
	if err != nil {
		return []wurl{}, err
	}

	raw, err := ioutil.ReadAll(res.Body)

	res.Body.Close()
	if err != nil {
		return []wurl{}, err
	}

	var wrapper [][]string
	err = json.Unmarshal(raw, &wrapper)

	out := make([]wurl, 0, len(wrapper))

	skip := true
	for _, urls := range wrapper {
		// The first item is always just the string "original",
		// so we should skip the first item
		if skip {
			skip = false
			continue
		}
		out = append(out, wurl{date: urls[1], url: urls[2]})
	}

	return out, nil

}

func getWaybackURLsAll(domain string, noSubs bool, days int) ([]wurl, error) {
	subsWildcard := "*."
	if noSubs {
		subsWildcard = ""
	}

	res, err := http.Get(
		fmt.Sprintf("http://web.archive.org/cdx/search/cdx?url=%s%s/*&output=json&collapse=urlkey", subsWildcard, domain),
	)
	if err != nil {
		return []wurl{}, err
	}

	raw, err := ioutil.ReadAll(res.Body)

	res.Body.Close()
	if err != nil {
		return []wurl{}, err
	}

	var wrapper [][]string
	err = json.Unmarshal(raw, &wrapper)

	out := make([]wurl, 0, len(wrapper))

	skip := true
	for _, urls := range wrapper {
		// The first item is always just the string "original",
		// so we should skip the first item
		if skip {
			skip = false
			continue
		}
		out = append(out, wurl{date: urls[1], url: urls[2]})
	}

	return out, nil

}

// Other functions remain unchanged
