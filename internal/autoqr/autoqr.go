package autoqr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

// GenerateQRPng creates a PNG file for the given url and returns its path.
func GenerateQRPng(url string, size int) (string, error) {
	if size <= 0 {
		size = 300
	}
	dir := os.TempDir()
	name := fmt.Sprintf("wzj_autoqr_%d.png", time.Now().UnixNano())
	out := filepath.Join(dir, name)
	if err := qrcode.WriteFile(url, qrcode.Medium, size, out); err != nil {
		return "", err
	}
	return out, nil
}

func findAutoHotkey() (string, error) {
	// Allow override
	if p := os.Getenv("AUTOHOTKEY_EXE"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	// PATH
	if p, err := exec.LookPath("AutoHotkey64.exe"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("AutoHotkey.exe"); err == nil {
		return p, nil
	}
	// Typical install dirs
	candidates := []string{
		`C:\\Program Files\\AutoHotkey\\AutoHotkey64.exe`,
		`C:\\Program Files\\AutoHotkey\\AutoHotkey.exe`,
		`C:\\Program Files (x86)\\AutoHotkey\\AutoHotkey.exe`,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("AutoHotkey 未找到，请安装或设置环境变量 AUTOHOTKEY_EXE")
}

// LaunchWeChatScreenshot shows the QR PNG in a topmost window via AutoHotkey,
// triggers WeChat screenshot (Alt+A), drags the selection over the image, and clicks center.
// x,y specify screen position; width/height are derived from pngSize.
func LaunchWeChatScreenshot(pngPath string, x, y, pngSize int) error {
	ahk, err := findAutoHotkey()
	if err != nil {
		return err
	}
	if pngSize <= 0 {
		pngSize = 300
	}
	// Build temporary AHK script (v1 syntax)
	script := fmt.Sprintf(`
#NoTrayIcon
#SingleInstance, Force
SetBatchLines, -1
CoordMode, Mouse, Screen
DllCall("SetProcessDPIAware")
x := %d
y := %d
w := %d
h := %d
img := "%s"
Gui, -DPIScale
Gui, +AlwaysOnTop -Caption +ToolWindow
Gui, Margin, 0, 0
Gui, Add, Picture, x0 y0 w%v h%v vPic, %s
Gui, Show, x%%x%% y%%y%% NoActivate, QRShow
Sleep, 150
; Activate WeChat (best-effort)
WinActivate, ahk_exe WeChat.exe
Sleep, 120
Send, !a
Sleep, 150
; Drag selection
tx := x + w
ty := y + h
MouseMove, %%x%%, %%y%%, 0
Sleep, 50
Click, down
Sleep, 60
MouseMove, %%tx%%, %%ty%%, 0
Sleep, 80
Click, up
Sleep, 300
; Click center to trigger recognition bubble
cx := x + w/2
cy := y + h/2
MouseClick, left, %%cx%%, %%cy%%
Sleep, 500
; Click absolute position for "识别二维码" icon/button
MouseClick, left, 560, 520
Sleep, 800
Gui, Destroy
ExitApp
`, x, y, pngSize, pngSize, escapeAHKPath(pngPath), pngSize, pngSize, escapeAHKPath(pngPath))

	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("wzj_wechat_autoqr_%d.ahk", time.Now().UnixNano()))
	if err := os.WriteFile(tmp, []byte(script), 0644); err != nil {
		return err
	}
	cmd := exec.Command(ahk, tmp)
	return cmd.Start()
}

func escapeAHKPath(p string) string {
	// AHK uses backslashes; ensure escaping quotes
	s := strings.ReplaceAll(p, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
