package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestUpdateProgress(t *testing.T) {
	for _, tc := range []struct {
		downloaded, total int64
		want              string
	}{
		{0, 0, "香篆 · 正在下载更新…"},
		{50, 100, "香篆 · 正在下载更新 50%"},
		{120, 100, "香篆 · 正在下载更新 100%"},
		{-1, 100, "香篆 · 正在下载更新 0%"},
	} {
		if got := updateProgress(tc.downloaded, tc.total); got != tc.want {
			t.Fatalf("updateProgress(%d, %d) = %q, want %q", tc.downloaded, tc.total, got, tc.want)
		}
	}
}

func TestUpdateCheckGuards(t *testing.T) {
	a := &app{}
	a.quitting.Store(true)
	a.checkUpdates(true)
	if a.updateBusy.Load() {
		t.Fatal("quitting must not start an update")
	}
	a.quitting.Store(false)
	a.updateBusy.Store(true)
	a.checkUpdates(true)
	if !a.updateBusy.Load() {
		t.Fatal("duplicate check must not release the active session")
	}
}

func TestUpdateRoutesPreferAListeningProxy(t *testing.T) {
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	open := "http://" + proxy.Addr().String()
	closed := freeAddr(t)

	if got := updateRoutes([]string{closed, open}); !slices.Equal(got, []string{open, ""}) {
		t.Fatalf("updateRoutes with a listening proxy = %q, want [%s ]", got, open)
	}
	if got := updateRoutes([]string{closed}); !slices.Equal(got, []string{""}) {
		t.Fatalf("updateRoutes without a proxy = %q, want a direct connection only", got)
	}
}

func TestUpdateTrafficUsesAListeningProxy(t *testing.T) {
	var proxied, direct atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied.Add(1)
		io.WriteString(w, "through the proxy")
	}))
	defer proxy.Close()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		direct.Add(1)
		io.WriteString(w, "straight to GitHub")
	}))
	defer target.Close()

	defer swapProxies([]string{proxy.URL})()

	if body, err := fetchThroughProxy(target.URL); err != nil || body != "through the proxy" {
		t.Fatalf("fetch = %q, %v; want the proxy's answer", body, err)
	}
	if proxied.Load() != 1 || direct.Load() != 0 {
		t.Fatalf("proxy saw %d requests, target saw %d, want 1 and 0", proxied.Load(), direct.Load())
	}
}

func TestUpdateTrafficFallsBackToADirectConnection(t *testing.T) {
	var proxied, direct atomic.Int32
	// A proxy that listens but cannot serve: the direct route is the only
	// way an update still gets through, and it is tried when the proxy
	// fails.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied.Add(1)
		http.Error(w, "no upstream", http.StatusBadGateway)
	}))
	defer proxy.Close()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		direct.Add(1)
		io.WriteString(w, "straight to GitHub")
	}))
	defer target.Close()

	defer swapProxies([]string{proxy.URL})()

	if body, err := fetchThroughProxy(target.URL); err != nil || body != "straight to GitHub" {
		t.Fatalf("fetch = %q, %v; want a direct answer", body, err)
	}
	if proxied.Load() != 1 || direct.Load() != 1 {
		t.Fatalf("proxy saw %d requests, target saw %d, want 1 and 1", proxied.Load(), direct.Load())
	}
}

func TestUpdateFailureNamesTheRoutesItTried(t *testing.T) {
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	open := "http://" + proxy.Addr().String()

	for _, tc := range []struct {
		name       string
		candidates []string
		want       string
	}{
		{"without a proxy", []string{freeAddr(t)}, "已尝试 直连："},
		{"with a listening proxy", []string{open}, "已尝试 本地代理 " + proxy.Addr().String() + "、直连："},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer swapProxies(tc.candidates)()
			failure := errors.New("connection reset")
			err := throughProxy(time.Second, func(context.Context) error { return failure })
			if !errors.Is(err, failure) {
				t.Fatalf("error = %v, want the underlying failure", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to start with %q", err, tc.want)
			}
		})
	}
}

func TestRouteClientRestoresTheDefaultClient(t *testing.T) {
	before := http.DefaultClient.Transport
	routeClient("")()
	if http.DefaultClient.Transport != before {
		t.Fatal("without a proxy the client must be left alone")
	}
	restore := routeClient("http://127.0.0.1:7890")
	if http.DefaultClient.Transport == before {
		t.Fatal("with a proxy the client must fetch through it")
	}
	restore()
	if http.DefaultClient.Transport != before {
		t.Fatal("the client must be restored afterwards")
	}
}

func TestRouteName(t *testing.T) {
	for route, want := range map[string]string{
		"":                        "直连",
		"http://127.0.0.1:7890":   "本地代理 127.0.0.1:7890",
		"socks5://127.0.0.1:7891": "本地代理 127.0.0.1:7891",
	} {
		if got := routeName(route); got != want {
			t.Fatalf("routeName(%q) = %q, want %q", route, got, want)
		}
	}
}

// fetchThroughProxy fetches url the way mygo's updater does: through
// http.DefaultClient, treating any status but 200 as a failure.
func fetchThroughProxy(url string) (string, error) {
	var body string
	err := throughProxy(5*time.Second, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GET %s: %s", url, resp.Status)
		}
		b, err := io.ReadAll(resp.Body)
		body = string(b)
		return err
	})
	return body, err
}

// swapProxies makes candidates the probed proxies until the returned
// function runs.
func swapProxies(candidates []string) func() {
	previous := updateProxyCandidates
	updateProxyCandidates = candidates
	return func() { updateProxyCandidates = previous }
}

// freeAddr is a loopback address nothing listens on.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}
