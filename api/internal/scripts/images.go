package scripts

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Images finds the download link for a template recipe's image. Most mirrors
// keep a link that always serves the current build; for the rest (Fedora,
// Alpine) it reads the mirror's listing from the Hypervane host and picks the
// newest build, falling back to the pinned one when the listing can't be read.
type Images struct {
	HTTP *http.Client
	TTL  time.Duration

	mu    sync.Mutex
	cache map[string]cachedImage
}

type cachedImage struct {
	file string
	at   time.Time
}

// Resolved is the image a template will be built from.
type Resolved struct {
	URL      string
	Filename string
	// Pinned is true when the newest build couldn't be looked up.
	Pinned bool
}

// NewImages returns a resolver with short timeouts, so a host without
// internet access falls back to the pinned build quickly.
func NewImages() *Images {
	return &Images{HTTP: &http.Client{Timeout: 8 * time.Second}, TTL: 6 * time.Hour}
}

func (im *Images) Resolve(ctx context.Context, img Image) Resolved {
	if img.Index == "" || img.Pattern == "" || im == nil {
		return Resolved{URL: img.URL, Filename: img.Filename}
	}
	im.mu.Lock()
	if c, ok := im.cache[img.Index]; ok && time.Since(c.at) < im.TTL {
		im.mu.Unlock()
		return Resolved{URL: img.Index + c.file, Filename: c.file}
	}
	im.mu.Unlock()

	file := im.newest(ctx, img)
	if file == "" {
		return Resolved{URL: img.URL, Filename: img.Filename, Pinned: true}
	}
	im.mu.Lock()
	if im.cache == nil {
		im.cache = map[string]cachedImage{}
	}
	im.cache[img.Index] = cachedImage{file: file, at: time.Now()}
	im.mu.Unlock()
	return Resolved{URL: img.Index + file, Filename: file}
}

func (im *Images) newest(ctx context.Context, img Image) string {
	re, err := regexp.Compile(img.Pattern)
	if err != nil {
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, img.Index, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Hypervane")
	client := im.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return ""
	}
	seen := map[string]bool{}
	var names []string
	for _, m := range re.FindAllString(string(body), -1) {
		if !seen[m] {
			seen[m] = true
			names = append(names, m)
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Slice(names, func(i, j int) bool { return versionLess(names[i], names[j]) })
	return names[len(names)-1]
}

// versionLess compares file names with their numbers as numbers, so 3.24.10
// sorts after 3.24.9 and 44-1.10 after 44-1.7.
func versionLess(a, b string) bool {
	ta, tb := versionTokens(a), versionTokens(b)
	for i := 0; i < len(ta) && i < len(tb); i++ {
		na, ea := strconv.Atoi(ta[i])
		nb, eb := strconv.Atoi(tb[i])
		switch {
		case ea == nil && eb == nil:
			if na != nb {
				return na < nb
			}
		case ta[i] != tb[i]:
			return ta[i] < tb[i]
		}
	}
	return len(ta) < len(tb)
}

var tokenRe = regexp.MustCompile(`[0-9]+|[^0-9]+`)

func versionTokens(s string) []string {
	return tokenRe.FindAllString(strings.ToLower(s), -1)
}
