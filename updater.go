package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/egoist/mygo"
)

// A single session covers checking, prompting and installing: automatic and
// manual checks cannot race to replace the same app.
func (a *app) checkUpdates(manual bool) {
	if a.quitting.Load() || !a.updateBusy.CompareAndSwap(false, true) {
		return
	}
	defer a.updateBusy.Store(false)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	up, err := mygo.Updater.Check(ctx)
	cancel()
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
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
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
	err = up.Install(ctx, func(downloaded, total int64) {
		if a.tray != nil {
			a.tray.SetToolTip(updateProgress(downloaded, total))
		}
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
