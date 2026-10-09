package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// Local proxies update traffic is sent through when one of them listens:
// Clash, mihomo and the clients built on them use these ports by default.
// The feed and the downloads live on GitHub, which is unreliable from
// mainland China, so a proxy is probed and preferred.
var updateProxyCandidates = []string{
	"http://127.0.0.1:7890",   // Clash, mihomo, ClashX: mixed (HTTP) port
	"socks5://127.0.0.1:7891", // their SOCKS port, when the mixed one is off
	"http://127.0.0.1:7897",   // Clash Verge Rev: mixed port
}

// updateRoutes is where update traffic is tried, in order: the first
// listening candidate, then a direct connection, which is the only route
// when no proxy listens.
func updateRoutes(candidates []string) []string {
	for _, raw := range candidates {
		u, err := url.Parse(raw)
		if err == nil && listens(u.Host) {
			return []string{raw, ""}
		}
	}
	return []string{""}
}

// listens reports whether something accepts TCP connections at addr.
func listens(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// throughProxy runs fn over each update route until one works, so an update
// gets through whether or not a local proxy is running. Each route gets its
// own timeout: a proxy that hangs must not use up the time the direct
// attempt needs.
func throughProxy(timeout time.Duration, fn func(ctx context.Context) error) error {
	var (
		tried []string
		err   error
	)
	for _, route := range updateRoutes(updateProxyCandidates) {
		restore := routeClient(route)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err = fn(ctx)
		cancel()
		restore()
		if err == nil {
			log.Printf("update over %s", routeName(route))
			return nil
		}
		tried = append(tried, routeName(route))
		log.Printf("update over %s failed: %v", routeName(route), err)
	}
	return fmt.Errorf("已尝试 %s：%w", strings.Join(tried, "、"), err)
}

// routeClient points what http.DefaultClient fetches — which is how mygo's
// updater reaches GitHub — at proxy until the returned function runs. An
// empty proxy leaves the client alone, so a direct connection still honours
// HTTPS_PROXY from the environment. One update session runs at a time, so
// swapping the client for the duration is safe.
func routeClient(proxy string) func() {
	u, err := url.Parse(proxy)
	base, ok := http.DefaultTransport.(*http.Transport)
	if proxy == "" || err != nil || !ok {
		return func() {}
	}
	transport := base.Clone()
	transport.Proxy = http.ProxyURL(u)
	previous := http.DefaultClient.Transport
	http.DefaultClient.Transport = transport
	return func() { http.DefaultClient.Transport = previous }
}

// routeName names a route, for the log and for what the failure dialog
// reports as tried.
func routeName(route string) string {
	if u, err := url.Parse(route); err == nil && u.Host != "" {
		return "本地代理 " + u.Host
	}
	return "直连"
}

// A single session covers checking, prompting and installing: automatic and
// manual checks cannot race to replace the same app.
func (a *app) checkUpdates(manual bool) {
	if a.quitting.Load() || !a.updateBusy.CompareAndSwap(false, true) {
		return
	}
	defer a.updateBusy.Store(false)
	var up *mygo.Update
	err := throughProxy(30*time.Second, func(ctx context.Context) error {
		var err error
		up, err = mygo.Updater.Check(ctx)
		return err
	})
	if err != nil {
		log.Printf("update check: %v", err)
		if manual {
			a.updateMessage("暂时无法检查更新", "请确认网络连接，且应用已安装在可写目录中（不要直接从 DMG 运行）。\n"+err.Error())
		}
		return
	}
	if up == nil {
		if manual {
			a.updateMessage("已是最新版本", "当前版本："+mygo.App.Version())
		}
		return
	}
	if a.quitting.Load() {
		return
	}
	result, err := mygo.Dialog.Message(mygo.MessageOptions{
		Title: "香篆更新", Message: "发现新版本 " + up.Version,
		Detail:  "下载完成后将重启香篆，当前计时会重新开始。",
		Buttons: []string{"下载并重启", "稍后"}, DefaultButton: 1, CancelButton: 1,
	})
	if err != nil || result.Button != 0 {
		return
	}
	a.updateDownloading.Store(true)
	defer a.updateDownloading.Store(false)
	if a.tray != nil {
		a.tray.SetToolTip("香篆 · 正在下载更新…")
		defer func() {
			a.mu.Lock()
			ph, left := a.phase, a.remaining
			a.mu.Unlock()
			a.tray.SetToolTip(trayTooltip(ph, left))
		}()
	}
	err = throughProxy(10*time.Minute, func(ctx context.Context) error {
		return up.Install(ctx, func(downloaded, total int64) {
			if a.tray != nil {
				a.tray.SetToolTip(updateProgress(downloaded, total))
			}
		})
	})
	if err != nil {
		log.Printf("update install: %v", err)
		a.updateMessage("更新未能完成", "请稍后重试。\n"+err.Error())
		return
	}
	mygo.App.Relaunch()
}

func updateProgress(downloaded, total int64) string {
	if total <= 0 {
		return "香篆 · 正在下载更新…"
	}
	percent := max(0, min(100, int(float64(downloaded)/float64(total)*100)))
	return fmt.Sprintf("香篆 · 正在下载更新 %d%%", percent)
}

func (a *app) updateMessage(message, detail string) {
	if a.quitting.Load() {
		return
	}
	if _, err := mygo.Dialog.Message(mygo.MessageOptions{
		Title: "香篆更新", Message: message, Detail: detail, Buttons: []string{"知道了"},
	}); err != nil {
		log.Printf("update dialog: %v", err)
	}
}
