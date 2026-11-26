// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func payload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + i/251)
	}
	return b
}

func quick(urls []string, dest, sha string) Request {
	return Request{URLs: urls, Dest: dest, SHA256: sha, Backoff: time.Millisecond, Attempts: 3, StallTimeout: 2 * time.Second}
}

func TestFileFetchesAndChecks(t *testing.T) {
	data := payload(300 << 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	var last int64
	req := quick([]string{srv.URL}, dest, sum(data))
	req.Progress = func(d, total int64) { last = d }
	res, err := File(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(data) || res.SHA256 != sum(data) || res.Size != int64(len(data)) || last != int64(len(data)) {
		t.Fatalf("result %+v, progress ended at %d", res, last)
	}
	if _, err := os.Stat(dest + ".part"); err == nil {
		t.Fatal("the .part file was left behind")
	}
}

func TestFileWrongSumIsThrownAway(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("not it")) }))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	_, err := File(context.Background(), quick([]string{srv.URL}, dest, sum([]byte("it"))))
	var fe *Error
	if !errors.As(err, &fe) || fe.Kind != Checksum || !strings.Contains(err.Error(), "not the expected file") {
		t.Fatalf("got %v", err)
	}
	for _, p := range []string{dest, dest + ".part"} {
		if _, e := os.Stat(p); e == nil {
			t.Errorf("%s was kept", p)
		}
	}
}

func TestFileBadSumArgument(t *testing.T) {
	if _, err := File(context.Background(), quick([]string{"http://x"}, filepath.Join(t.TempDir(), "f"), "nothex")); err == nil {
		t.Fatal("a malformed SHA-256 was accepted")
	}
	if _, err := File(context.Background(), Request{Dest: "f"}); err == nil {
		t.Fatal("no address was accepted")
	}
}

func TestFileStatusErrorsAndMirrors(t *testing.T) {
	data := payload(5000)
	var gone, flaky int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gone":
			atomic.AddInt32(&gone, 1)
			http.NotFound(w, r)
		case "/flaky":
			if atomic.AddInt32(&flaky, 1) < 3 {
				http.Error(w, "busy", http.StatusServiceUnavailable)
				return
			}
			w.Write(data)
		default:
			w.Write(data)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()

	// a 404 is not retried
	_, err := File(context.Background(), quick([]string{srv.URL + "/gone"}, filepath.Join(dir, "a"), ""))
	var fe *Error
	if !errors.As(err, &fe) || fe.Kind != Status || fe.Status != 404 || atomic.LoadInt32(&gone) != 1 {
		t.Fatalf("404: %v after %d tries", err, gone)
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("the message does not say 404: %v", err)
	}
	// a 503 is retried until it works
	if _, err := File(context.Background(), quick([]string{srv.URL + "/flaky"}, filepath.Join(dir, "b"), sum(data))); err != nil || flaky != 3 {
		t.Fatalf("flaky: %v after %d tries", err, flaky)
	}
	// the second address is used when the first fails, and the result says which
	res, err := File(context.Background(), quick([]string{srv.URL + "/gone", srv.URL + "/ok"}, filepath.Join(dir, "c"), sum(data)))
	if err != nil || !strings.HasSuffix(res.URL, "/ok") {
		t.Fatalf("mirror: %+v, %v", res, err)
	}
	// when all fail the message counts the others
	_, err = File(context.Background(), quick([]string{srv.URL + "/gone", srv.URL + "/gone"}, filepath.Join(dir, "d"), ""))
	if err == nil || !strings.Contains(err.Error(), "1 other address") {
		t.Fatalf("all failing: %v", err)
	}
}

func TestFileNoConnectionSaysSo(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens there now
	_, err := File(context.Background(), quick([]string{url}, filepath.Join(t.TempDir(), "f"), ""))
	var fe *Error
	if !errors.As(err, &fe) || fe.Kind != Network || !strings.Contains(err.Error(), "connected to the internet") {
		t.Fatalf("got %v", err)
	}
}

// rangeServer serves data with Range support, but the first cutAfter bytes only, once.
func rangeServer(data []byte, cutAfter int, ranges *[]string) *httptest.Server {
	var cut int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*ranges = append(*ranges, r.Header.Get("Range"))
		if r.Header.Get("Range") == "" && atomic.AddInt32(&cut, 1) == 1 {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Write(data[:cutAfter])
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler) // the connection drops
		}
		http.ServeContent(w, r, "f", time.Time{}, strings.NewReader(string(data)))
	}))
}

func TestFileResumesACutDownload(t *testing.T) {
	data := payload(400 << 10)
	var ranges []string
	srv := rangeServer(data, 150<<10, &ranges)
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	if _, err := File(context.Background(), quick([]string{srv.URL}, dest, sum(data))); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(data) {
		t.Fatal("the resumed file differs")
	}
	if len(ranges) != 2 || ranges[0] != "" || !strings.HasPrefix(ranges[1], "bytes=") || strings.HasSuffix(ranges[1], "=0-") {
		t.Fatalf("requests carried %q: the second should resume part way", ranges)
	}
}

func TestFileResumesAcrossRuns(t *testing.T) {
	data := payload(200 << 10)
	var ranges []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("Range"))
		http.ServeContent(w, r, "f", time.Time{}, strings.NewReader(string(data)))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	os.WriteFile(dest+".part", data[:50000], 0o644) // what an earlier, interrupted run left
	if _, err := File(context.Background(), quick([]string{srv.URL}, dest, sum(data))); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) != string(data) || ranges[0] != "bytes=50000-" {
		t.Fatalf("ranges %q", ranges)
	}
}

func TestFileDistrustsAPartItCannotCheck(t *testing.T) {
	data := payload(10000)
	var ranges []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("Range"))
		http.ServeContent(w, r, "f", time.Time{}, strings.NewReader(string(data)))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	os.WriteFile(dest+".part", []byte("from some other file"), 0o644)
	if _, err := File(context.Background(), quick([]string{srv.URL}, dest, "")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) != string(data) || ranges[0] != "" {
		t.Fatalf("an unchecked download resumed a part of unknown origin (ranges %q)", ranges)
	}
}

func TestFileStartsAgainWhenAResumedPartWasWrong(t *testing.T) {
	data := payload(100 << 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "f", time.Time{}, strings.NewReader(string(data)))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	os.WriteFile(dest+".part", []byte("garbage that is not the start of the file"), 0o644)
	if _, err := File(context.Background(), quick([]string{srv.URL}, dest, sum(data))); err != nil {
		t.Fatalf("a wrong part was not recovered from: %v", err)
	}
	if got, _ := os.ReadFile(dest); string(got) != string(data) {
		t.Fatal("wrong contents")
	}
}

func TestFileReusesWhatIsThere(t *testing.T) {
	data := payload(1000)
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&n, 1); w.Write(data) }))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	os.WriteFile(dest, data, 0o644)
	res, err := File(context.Background(), quick([]string{srv.URL}, dest, sum(data)))
	if err != nil || !res.Reused || n != 0 {
		t.Fatalf("%+v, %v, %d requests", res, err, n)
	}
	// a file that is there but is not the one wanted is fetched over
	os.WriteFile(dest, []byte("old"), 0o644)
	if res, err := File(context.Background(), quick([]string{srv.URL}, dest, sum(data))); err != nil || res.Reused || n != 1 {
		t.Fatalf("%+v, %v, %d requests", res, err, n)
	}
}

func TestFileGivesUpOnAStall(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000")
		w.Write([]byte("a little"))
		w.(http.Flusher).Flush()
		<-release
	}))
	defer srv.Close()
	defer close(release)
	req := quick([]string{srv.URL}, filepath.Join(t.TempDir(), "f"), "")
	req.Attempts, req.StallTimeout = 1, 150*time.Millisecond
	start := time.Now()
	_, err := File(context.Background(), req)
	var fe *Error
	if !errors.As(err, &fe) || fe.Kind != Network || !strings.Contains(err.Error(), "stopped coming") {
		t.Fatalf("got %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("the stall took too long to be noticed")
	}
}

func TestFileCanBeStopped(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000")
		w.Write([]byte("a little"))
		w.(http.Flusher).Flush()
		<-release
	}))
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	_, err := File(ctx, quick([]string{srv.URL}, filepath.Join(t.TempDir(), "f"), ""))
	var fe *Error
	if !errors.As(err, &fe) || fe.Kind != Stopped {
		t.Fatalf("got %v", err)
	}
}

func TestNewClientUsesTheEnvironmentsProxy(t *testing.T) {
	tr, ok := NewClient().Transport.(*http.Transport)
	if !ok || tr.Proxy == nil {
		t.Fatal("the client does not take the proxy from the environment")
	}
	if NewClient().Timeout != 0 {
		t.Fatal("a download must have no overall timeout")
	}
}
