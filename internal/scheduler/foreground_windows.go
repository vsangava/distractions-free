//go:build windows

package scheduler

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows foreground probe — the "start simple" path for issue #94. It reads:
//
//   - the foreground window (GetForegroundWindow)
//   - the process behind it, to decide if it's a tracked browser
//     (GetWindowThreadProcessId → OpenProcess → QueryFullProcessImageName)
//   - the window title, and a best-effort host from it (hostFromBrowserWindowTitle)
//   - idle time (GetLastInputInfo vs GetTickCount — the Win32 analogue of macOS
//     HIDIdleTime)
//
// It does NOT read the real address-bar URL — that needs UI Automation and
// lands in a follow-up. Until then this populates foreground_minutes only for
// the subset of pages whose window title contains a domain.

var (
	modUser32   = windows.NewLazySystemDLL("user32.dll")
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetForegroundWindow      = modUser32.NewProc("GetForegroundWindow")
	procGetWindowTextW           = modUser32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = modUser32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
	procGetLastInputInfo         = modUser32.NewProc("GetLastInputInfo")
	procGetTickCount             = modKernel32.NewProc("GetTickCount")
)

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

// windowsBrowserExeToName maps a foreground process's executable basename
// (lowercased) to the canonical browser name used in supportedForegroundBrowsers
// and emitted in usage events. Chrome + Edge only for now (issue #94 scope).
var windowsBrowserExeToName = map[string]string{
	"chrome.exe": "Google Chrome",
	"msedge.exe": "Microsoft Edge",
}

func init() { foregroundProbe = windowsForegroundProbe{} }

type windowsForegroundProbe struct{}

func (windowsForegroundProbe) Probe() (ForegroundProbeResult, error) {
	idle, err := windowsIdleSeconds()
	if err != nil {
		return ForegroundProbeResult{}, err
	}
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		// Locked screen, secure desktop (UAC), or no active window.
		return ForegroundProbeResult{IdleSeconds: idle}, nil
	}
	exe := strings.ToLower(windowsForegroundExeBaseName(hwnd))
	browser, ok := windowsBrowserExeToName[exe]
	if !ok {
		return ForegroundProbeResult{IdleSeconds: idle}, nil
	}
	url := hostFromBrowserWindowTitle(windowsWindowText(hwnd)) // "" when nothing extractable
	return ForegroundProbeResult{App: browser, URL: url, IdleSeconds: idle}, nil
}

// windowsIdleSeconds returns how long since the last keyboard/mouse input.
// GetLastInputInfo.dwTime and GetTickCount are both 32-bit ms-since-boot
// counters that wrap roughly every 49.7 days; uint32 subtraction handles a
// single wrap correctly.
func windowsIdleSeconds() (int, error) {
	lii := lastInputInfo{}
	lii.cbSize = uint32(unsafe.Sizeof(lii))
	r, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lii)))
	if r == 0 {
		return 0, fmt.Errorf("GetLastInputInfo failed: %w", err)
	}
	tick, _, _ := procGetTickCount.Call()
	elapsedMs := uint32(tick) - lii.dwTime
	return int(elapsedMs / 1000), nil
}

// windowsWindowText returns the title bar text of hwnd ("" if none).
func windowsWindowText(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

// windowsForegroundExeBaseName returns the executable basename (e.g.
// "chrome.exe") of the process owning hwnd, or "" on any failure. Best-effort:
// errors here just mean "treat as not-a-tracked-browser", never fatal.
func windowsForegroundExeBaseName(hwnd uintptr) string {
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	full := windows.UTF16ToString(buf[:size])
	if i := strings.LastIndexAny(full, `\/`); i >= 0 {
		return full[i+1:]
	}
	return full
}
