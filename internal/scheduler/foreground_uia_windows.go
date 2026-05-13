//go:build windows

package scheduler

import (
	"log"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	ole "github.com/go-ole/go-ole"
)

// UI Automation address-bar reader for the Windows foreground probe — the
// accurate path that replaces the window-title heuristic when
// `windows_foreground_use_uia` is on. It walks the foreground window's UIA tree
// for the Chromium "Address and search bar" Edit control and reads its Value.
//
// This is hand-rolled COM: go-ole gives us CoInitialize / CoCreateInstance /
// IUnknown, but the UIAutomation interfaces aren't IDispatch-friendly, so the
// vtables and method calls below are defined manually. The struct field order
// MUST exactly match UIAutomationClient.h — a wrong offset calls a garbage
// function pointer. Every call is null-checked, COM objects are released, and
// the whole thing is wrapped in a recover() so a bad call (or a flaky browser
// UIA tree) degrades to the window-title fallback instead of taking down the
// daemon.
//
// NOTE: cross-compiled and reviewed but not yet exercised on a real Windows
// box. Until it has mileage it's opt-in (default false); see issue #94.

var (
	clsidCUIAutomation = ole.NewGUID("{ff48dba4-60ef-4201-aa87-54103eef594e}")
	iidIUIAutomation   = ole.NewGUID("{30cbe57d-d9d0-452a-ab13-7ac5ac4825ee}")
)

// Well-known UIA property / control-type IDs (UIAutomationClient.h).
const (
	uiaControlTypePropertyID = 30003
	uiaNamePropertyID        = 30005
	uiaValueValuePropertyID  = 30045
	uiaEditControlTypeID     = 50004
	treeScopeDescendants     = 4
)

// omniboxNames are the localized accessibility Name values Chromium gives the
// address-bar Edit control. English is the floor; the rest are best-effort —
// a string that doesn't match just falls through to the next, and if none
// match the caller uses the window-title heuristic. (Non-English coverage and
// other browsers/Firefox are a follow-up.)
var omniboxNames = []string{
	"Address and search bar",
	"Address bar",
	"Adress- und Suchleiste",             // de
	"Barre d'adresse et de recherche",    // fr
	"Barra de direcciones y de búsqueda", // es
	"Barra de endereço e pesquisa",       // pt-BR
}

// --- minimal UIAutomation COM interfaces ------------------------------------

type iUIAutomationVtbl struct {
	ole.IUnknownVtbl
	CompareElements                     uintptr
	CompareRuntimeIds                   uintptr
	GetRootElement                      uintptr
	GetFocusedElement                   uintptr
	GetRootElementBuildCache            uintptr
	GetFocusedElementBuildCache         uintptr
	GetPatternProgrammaticName          uintptr
	GetPropertyProgrammaticName         uintptr
	PollForPotentialSupportedPatterns   uintptr
	PollForPotentialSupportedProperties uintptr
	CheckNotSupported                   uintptr
	GetReservedNotSupportedValue        uintptr
	GetReservedMixedAttributeValue      uintptr
	ElementFromHandle                   uintptr
	ElementFromPoint                    uintptr
	ElementFromIAccessible              uintptr
	ElementFromIAccessibleBuildCache    uintptr
	CreateCacheRequest                  uintptr
	CreateTrueCondition                 uintptr
	CreateFalseCondition                uintptr
	CreatePropertyCondition             uintptr
	CreatePropertyConditionEx           uintptr
	CreateAndCondition                  uintptr
	// (remaining methods unused)
}

type iUIAutomation struct{ ole.IUnknown }

func (a *iUIAutomation) vtbl() *iUIAutomationVtbl {
	return (*iUIAutomationVtbl)(unsafe.Pointer(a.RawVTable))
}

func (a *iUIAutomation) elementFromHandle(hwnd uintptr) *iUIAutomationElement {
	var el *iUIAutomationElement
	hr, _, _ := syscall.SyscallN(a.vtbl().ElementFromHandle, uintptr(unsafe.Pointer(a)), hwnd, uintptr(unsafe.Pointer(&el)))
	if hr != 0 || el == nil {
		return nil
	}
	return el
}

func (a *iUIAutomation) createPropertyCondition(propertyID int32, v ole.VARIANT) *iUIAutomationCondition {
	var cond *iUIAutomationCondition
	hr, _, _ := syscall.SyscallN(a.vtbl().CreatePropertyCondition,
		uintptr(unsafe.Pointer(a)), uintptr(propertyID), uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&cond)))
	if hr != 0 || cond == nil {
		return nil
	}
	return cond
}

func (a *iUIAutomation) createAndCondition(c1, c2 *iUIAutomationCondition) *iUIAutomationCondition {
	var cond *iUIAutomationCondition
	hr, _, _ := syscall.SyscallN(a.vtbl().CreateAndCondition,
		uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(c1)), uintptr(unsafe.Pointer(c2)), uintptr(unsafe.Pointer(&cond)))
	if hr != 0 || cond == nil {
		return nil
	}
	return cond
}

type iUIAutomationElementVtbl struct {
	ole.IUnknownVtbl
	SetFocus                uintptr
	GetRuntimeId            uintptr
	FindFirst               uintptr
	FindAll                 uintptr
	FindFirstBuildCache     uintptr
	FindAllBuildCache       uintptr
	BuildUpdatedCache       uintptr
	GetCurrentPropertyValue uintptr
	// (remaining methods unused)
}

type iUIAutomationElement struct{ ole.IUnknown }

func (e *iUIAutomationElement) vtbl() *iUIAutomationElementVtbl {
	return (*iUIAutomationElementVtbl)(unsafe.Pointer(e.RawVTable))
}

func (e *iUIAutomationElement) findFirst(scope int32, cond *iUIAutomationCondition) *iUIAutomationElement {
	var found *iUIAutomationElement
	hr, _, _ := syscall.SyscallN(e.vtbl().FindFirst,
		uintptr(unsafe.Pointer(e)), uintptr(scope), uintptr(unsafe.Pointer(cond)), uintptr(unsafe.Pointer(&found)))
	if hr != 0 || found == nil {
		return nil
	}
	return found
}

func (e *iUIAutomationElement) currentPropertyValueString(propertyID int32) string {
	var v ole.VARIANT
	hr, _, _ := syscall.SyscallN(e.vtbl().GetCurrentPropertyValue, uintptr(unsafe.Pointer(e)), uintptr(propertyID), uintptr(unsafe.Pointer(&v)))
	if hr != 0 {
		return ""
	}
	defer ole.VariantClear(&v)
	return strings.TrimSpace(v.ToString())
}

// iUIAutomationCondition is opaque to us — we only ever create, pass, and
// release these — so it's just an IUnknown.
type iUIAutomationCondition struct{ ole.IUnknown }

// --- the actual probe -------------------------------------------------------

// windowsAddressBarURL reads the active tab's address-bar value from a Chromium
// browser (Chrome/Edge) window via UI Automation. Returns "" on any failure —
// COM init, missing element, non-omnibox UI locale, or a panic from a bad
// vtable call — and the caller falls back to the window-title heuristic.
//
// The value returned is whatever the omnibox *displays*: that equals the loaded
// URL except while the user is mid-edit (typing a new address). normalizeAddressBarValue
// re-attaches a scheme when Chromium has elided it.
func windowsAddressBarURL(hwnd uintptr) (result string) {
	if hwnd == 0 {
		return ""
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("scheduler: windows UIA address-bar probe panicked, using window-title fallback: %v", r)
			result = ""
		}
	}()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitialize(0); err != nil {
		return ""
	}
	defer ole.CoUninitialize()

	unk, err := ole.CreateInstance(clsidCUIAutomation, iidIUIAutomation)
	if err != nil || unk == nil {
		return ""
	}
	automation := (*iUIAutomation)(unsafe.Pointer(unk))
	defer automation.Release()

	root := automation.elementFromHandle(hwnd)
	if root == nil {
		return ""
	}
	defer root.Release()

	editCond := automation.createPropertyCondition(uiaControlTypePropertyID, ole.NewVariant(ole.VT_I4, int64(uiaEditControlTypeID)))
	if editCond == nil {
		return ""
	}
	defer editCond.Release()

	for _, name := range omniboxNames {
		bstr := ole.SysAllocString(name)
		nameCond := automation.createPropertyCondition(uiaNamePropertyID, ole.NewVariant(ole.VT_BSTR, int64(uintptr(unsafe.Pointer(bstr)))))
		ole.SysFreeString(bstr)
		if nameCond == nil {
			continue
		}
		andCond := automation.createAndCondition(editCond, nameCond)
		nameCond.Release()
		if andCond == nil {
			continue
		}
		el := root.findFirst(treeScopeDescendants, andCond)
		andCond.Release()
		if el == nil {
			continue
		}
		val := el.currentPropertyValueString(uiaValueValuePropertyID)
		el.Release()
		if val != "" {
			return normalizeAddressBarValue(val)
		}
	}
	return ""
}
