// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

// Package fetch downloads files for ZenZX: the Next's boot files, the Python the examples
// need, and anything else a program or a launcher has to bring in. It is the one downloader
// of the program, and it does the work with Go's own network and TLS code, so it needs no
// curl, wget or PowerShell on the computer, and works on a Windows 7 that has none of them
// (nor the TLS 1.2 that Windows 7 leaves off for its own tools).
//
// What it does that a plain GET does not:
//
//   - a download goes to a ".part" file, is checked against its SHA-256 before it is given its
//     name, and a file that does not match is thrown away, never kept;
//   - a download that is cut off, or that an unreliable network drops, is resumed where it
//     stopped (when the server can), and tried again a few times, with a pause that grows;
//   - a download that stops sending is given up on after a while, instead of waiting for ever;
//   - there can be several addresses for one file, tried in turn;
//   - the proxy of the environment (HTTPS_PROXY and the others) is used;
//   - a failure says, in plain words, what went wrong: no connection, the server's answer,
//     or a file that is not the one expected.
//
// Unpack takes a zip or a .tar.gz apart safely, and Recipe names what ZenZX knows how to
// fetch (see recipe.go).
package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Kind says what kind of thing went wrong, for a caller that wants to word it differently.
type Kind int

const (
	// Network: the server could not be reached, or the connection broke, or it went quiet.
	Network Kind = iota
	// Status: the server answered, and the answer was not the file.
	Status
	// Checksum: what came is not the file that was expected.
	Checksum
	// Disk: the file could not be written.
	Disk
	// Stopped: the caller stopped the download.
	Stopped
)

// Error is what File returns. Its text is plain enough to show a person as it is.
type Error struct {
	Kind   Kind
	URL    string
	Status int    // the HTTP status, for Status
	Got    string // the SHA-256 that came, for Checksum
	Want   string // the SHA-256 that was expected, for Checksum
	Err    error  // the underlying error, for Network and Disk
	More   int    // how many other addresses were tried before this one, and failed
}

func (e *Error) Error() string {
	host := e.URL
	if u, err := url.Parse(e.URL); err == nil && u.Host != "" {
		host = u.Host
	}
	var s string
	switch e.Kind {
	case Status:
		s = fmt.Sprintf("%s answered %d %s for %s", host, e.Status, http.StatusText(e.Status), e.URL)
	case Checksum:
		s = fmt.Sprintf("what came from %s is not the expected file (its SHA-256 is %s, not %s), so it was thrown away", e.URL, e.Got, e.Want)
	case Disk:
		s = fmt.Sprintf("the download from %s could not be written: %v", e.URL, e.Err)
	case Stopped:
		s = "the download was stopped"
	default:
		s = fmt.Sprintf("could not get %s: %s. Is the computer connected to the internet?", e.URL, plainCause(e.Err))
	}
	if e.More > 0 {
		s += fmt.Sprintf(" (%d other address(es) had failed before it)", e.More)
	}
	return s
}

func (e *Error) Unwrap() error { return e.Err }

// plainCause words a network error without Go's own long prefixes.
func plainCause(err error) string {
	if err == nil {
		return "unknown"
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "no answer in time"
	}
	if errors.Is(err, errStalled) {
		return "the download stopped coming"
	}
	return err.Error()
}

var errStalled = errors.New("stalled")

// Request is one file to fetch.
type Request struct {
	URLs     []string // tried in order; the first that gives the file wins
	Dest     string   // where the file goes, once it is whole and checked
	SHA256   string   // hex; empty means the contents are not checked
	Progress func(done, total int64)

	// Client is what makes the requests; nil is NewClient(). Its Timeout must be 0: a download
	// has no overall limit, only StallTimeout.
	Client *http.Client
	// Attempts is how many times each address is tried when the network fails; 0 means 4.
	Attempts int
	// Backoff is the pause before the second attempt, doubled before each after; 0 means 1s.
	Backoff time.Duration
	// StallTimeout is how long a download may send nothing before it is given up on; 0 means 30s.
	StallTimeout time.Duration
	// UserAgent is sent with each request; empty means "zenzx".
	UserAgent string
}

// Result says what File fetched.
type Result struct {
	URL    string // the address it came from
	SHA256 string // of the file, hex
	Size   int64
	Reused bool // the file was already there with the expected SHA-256, and nothing was downloaded
}

// NewClient is the client File uses when it is given none: Go's own TLS, the environment's
// proxy, a limit on how long a connection may take to be made, and none on the download.
func NewClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = http.ProxyFromEnvironment
	tr.DialContext = (&net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	tr.TLSHandshakeTimeout = 20 * time.Second
	tr.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: tr}
}

// File fetches the file described by req. It is safe to call again after a failure: what a
// cut-off download left is picked up where it stopped.
func File(ctx context.Context, req Request) (Result, error) {
	if len(req.URLs) == 0 {
		return Result{}, errors.New("fetch: no address to fetch from")
	}
	want := strings.ToLower(strings.TrimSpace(req.SHA256))
	if want != "" && (len(want) != 64 || !isHex(want)) {
		return Result{}, fmt.Errorf("fetch: %q is not a SHA-256", req.SHA256)
	}
	if want != "" {
		if sum, size, err := sumOf(req.Dest); err == nil && sum == want {
			return Result{URL: "", SHA256: sum, Size: size, Reused: true}, nil
		}
	}
	if want == "" {
		// Nothing says a part that is already there belongs to this file, so it is not trusted.
		os.Remove(req.Dest + ".part")
	}
	if req.Client == nil {
		req.Client = NewClient()
	}
	if req.Attempts <= 0 {
		req.Attempts = 4
	}
	if req.Backoff <= 0 {
		req.Backoff = time.Second
	}
	if req.StallTimeout <= 0 {
		req.StallTimeout = 30 * time.Second
	}
	if req.UserAgent == "" {
		req.UserAgent = "zenzx"
	}
	if req.Progress == nil {
		req.Progress = func(int64, int64) {}
	}

	var last *Error
	for i, u := range req.URLs {
		res, err := fetchOne(ctx, req, u, want)
		if err == nil {
			return res, nil
		}
		if err.Kind == Stopped {
			return Result{}, err
		}
		err.More = i
		last = err
	}
	return Result{}, last
}

// fetchOne gets the file from one address, with the retries.
func fetchOne(ctx context.Context, req Request, u, want string) (Result, *Error) {
	part := req.Dest + ".part"
	var lastErr *Error
	pause := req.Backoff
	for attempt := 1; attempt <= req.Attempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return Result{}, &Error{Kind: Stopped, URL: u, Err: ctx.Err()}
			case <-time.After(pause):
			}
			pause *= 2
		}
		res, err, retry := attemptOnce(ctx, req, u, part, want)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if !retry || ctx.Err() != nil {
			break
		}
	}
	if ctx.Err() != nil {
		return Result{}, &Error{Kind: Stopped, URL: u, Err: ctx.Err()}
	}
	return Result{}, lastErr
}

// attemptOnce is one try: it resumes the ".part" file when there is one. retry says whether
// another try could do better.
func attemptOnce(ctx context.Context, req Request, u, part, want string) (Result, *Error, bool) {
	var off int64
	if st, err := os.Stat(part); err == nil && !st.IsDir() {
		off = st.Size()
	}
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()
	hreq, err := http.NewRequestWithContext(rctx, http.MethodGet, u, nil)
	if err != nil {
		return Result{}, &Error{Kind: Network, URL: u, Err: err}, false
	}
	hreq.Header.Set("User-Agent", req.UserAgent)
	if off > 0 {
		hreq.Header.Set("Range", "bytes="+strconv.FormatInt(off, 10)+"-")
	}
	resp, err := req.Client.Do(hreq)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, &Error{Kind: Stopped, URL: u, Err: ctx.Err()}, false
		}
		return Result{}, &Error{Kind: Network, URL: u, Err: err}, true
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		off = 0 // the whole file comes, whatever was asked
	case resp.StatusCode == http.StatusPartialContent && off > 0 && contentRangeStart(resp.Header.Get("Content-Range")) == off:
		// it carries on from the part that is there
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		os.Remove(part) // what is there is not part of the file: start again
		return Result{}, &Error{Kind: Status, URL: u, Status: resp.StatusCode}, true
	default:
		retry := resp.StatusCode >= 500 || resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests
		return Result{}, &Error{Kind: Status, URL: u, Status: resp.StatusCode}, retry
	}

	hash := sha256.New()
	flags := os.O_CREATE | os.O_WRONLY
	if off > 0 {
		// The hash must cover the whole file, so the part that is there is read into it first.
		f, err := os.Open(part)
		if err != nil {
			return Result{}, &Error{Kind: Disk, URL: u, Err: err}, false
		}
		_, err = io.CopyN(hash, f, off)
		f.Close()
		if err != nil {
			os.Remove(part)
			return Result{}, &Error{Kind: Disk, URL: u, Err: err}, true
		}
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	out, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return Result{}, &Error{Kind: Disk, URL: u, Err: err}, false
	}

	total := resp.ContentLength
	if total >= 0 {
		total += off
	} else {
		total = 0
	}
	done := off
	// A download that goes quiet is cancelled by this timer, which every block of data resets.
	stall := time.AfterFunc(req.StallTimeout, cancel)
	defer stall.Stop()
	buf := make([]byte, 128*1024)
	var rerr error
	for {
		n, e := resp.Body.Read(buf)
		if n > 0 {
			stall.Reset(req.StallTimeout)
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				os.Remove(part)
				return Result{}, &Error{Kind: Disk, URL: u, Err: werr}, false
			}
			hash.Write(buf[:n])
			done += int64(n)
			req.Progress(done, total)
		}
		if e != nil {
			rerr = e
			break
		}
	}
	if cerr := out.Close(); cerr != nil {
		os.Remove(part)
		return Result{}, &Error{Kind: Disk, URL: u, Err: cerr}, false
	}
	if rerr != io.EOF {
		if ctx.Err() != nil {
			return Result{}, &Error{Kind: Stopped, URL: u, Err: ctx.Err()}, false
		}
		if rctx.Err() != nil { // the stall timer
			rerr = errStalled
		}
		return Result{}, &Error{Kind: Network, URL: u, Err: rerr}, true // the part is kept, to be resumed
	}
	if total > 0 && done != total {
		return Result{}, &Error{Kind: Network, URL: u, Err: fmt.Errorf("the download stopped at %d of %d bytes", done, total)}, true
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if want != "" && got != want {
		os.Remove(part)
		// A mismatch after a resumed download may be a part that was never this file: one more try, from the start.
		return Result{}, &Error{Kind: Checksum, URL: u, Got: got, Want: want}, off > 0
	}
	if err := os.Rename(part, req.Dest); err != nil {
		os.Remove(part)
		return Result{}, &Error{Kind: Disk, URL: u, Err: err}, false
	}
	return Result{URL: u, SHA256: got, Size: done}, nil, false
}

// contentRangeStart is the first byte of a "bytes 100-199/200" header, or -1.
func contentRangeStart(h string) int64 {
	h = strings.TrimPrefix(strings.TrimSpace(h), "bytes ")
	dash := strings.IndexByte(h, '-')
	if dash < 0 {
		return -1
	}
	n, err := strconv.ParseInt(h[:dash], 10, 64)
	if err != nil {
		return -1
	}
	return n
}

func isHex(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// sumOf is the SHA-256 and size of a file.
func sumOf(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
