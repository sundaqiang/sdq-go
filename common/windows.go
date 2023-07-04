//go:build windows

package common

import (
	"github.com/beevik/ntp"
	"go.uber.org/zap"
	"golang.org/x/sys/windows/registry"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	MaxPath           = 260
	EsContinuous      = 0x80000000
	EsSystemRequired  = 0x00000001
	EsDisplayRequired = 0x00000002
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procCreateToolHelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = kernel32.NewProc("Process32FirstW")
	procProcess32Next            = kernel32.NewProc("Process32NextW")
	setThreadExecutionStateProc  = kernel32.NewProc("SetThreadExecutionState")
	setSystemTime                = kernel32.NewProc("SetSystemTime")
)

type WindowsProcess struct {
	Pid        int
	PPid       int
	Executable string
}

type ProcessEntry32 struct {
	Size              uint32
	CntUsage          uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	CntThreads        uint32
	ParentProcessID   uint32
	PriorityClassBase int32
	Flags             uint32
	ExeFile           [MaxPath]uint16
}

type windowsTime struct {
	wYear         uint16
	wMonth        uint16
	wDayOfWeek    uint16
	wDay          uint16
	wHour         uint16
	wMinute       uint16
	wSecond       uint16
	wMilliseconds uint16
}

// 通过注册表获取已安装的软件
func getProduct(key uint32) ([]string, error) {
	const RegistryPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
	var softwareList []string

	// registry.WOW64_32KEY
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, RegistryPath, registry.ENUMERATE_SUB_KEYS|key)
	if err != nil {
		return nil, err
	}
	defer k.Close()

	keyNames, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}

	for _, subkeyName := range keyNames {
		subkey, err := registry.OpenKey(k, subkeyName, registry.READ|key)
		if err != nil {
			continue
		}

		displayName, _, err := subkey.GetStringValue("DisplayName")
		subkey.Close()

		if err != nil || len(displayName) == 0 {
			continue
		}

		softwareList = append(softwareList, displayName)
	}

	return softwareList, nil
}

// 系统休眠函数
func setThreadExecutionState(stateFlags uintptr) uintptr {
	ret, _, _ := setThreadExecutionStateProc.Call(stateFlags)
	return ret
}

// 将ProcessEntry32转换为WindowsProcess
func newWindowsProcess(e *ProcessEntry32) *WindowsProcess {
	// Find when the string ends for decoding
	end := 0
	for {
		if e.ExeFile[end] == 0 {
			break
		}
		end++
	}

	return &WindowsProcess{
		Pid:        int(e.ProcessID),
		PPid:       int(e.ParentProcessID),
		Executable: syscall.UTF16ToString(e.ExeFile[:end]),
	}
}

// AllProcesses 获取进程列表
func AllProcesses() []*WindowsProcess {
	handle, _, _ := procCreateToolHelp32Snapshot.Call(
		0x00000002,
		0)
	if handle < 0 {
		ZapLog.Error("获取进程列表错误",
			zap.Error(syscall.GetLastError()),
		)
		return nil
	}
	defer procCloseHandle.Call(handle)

	var entry ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	ret, _, _ := procProcess32First.Call(handle, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		ZapLog.Error("获取进程列表失败",
			zap.Error(syscall.GetLastError()),
		)
		return nil
	}

	results := make([]*WindowsProcess, 0, 50)
	for {
		results = append(results, newWindowsProcess(&entry))

		ret, _, _ := procProcess32Next.Call(handle, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	return results
}

// FindProcess 根据进程id获取进程信息
func FindProcess(pid int) *WindowsProcess {
	ps := AllProcesses()
	if ps == nil {
		return nil
	}
	for _, p := range ps {
		if p.Pid == pid {
			return p
		}
	}

	return nil
}

// AddHosts 添加hosts
func AddHosts(ip string, domain string) {
	hostsFilePath := "C:\\Windows\\System32\\drivers\\etc\\hosts"
	// 读取hosts文件
	content, err := os.ReadFile(hostsFilePath)
	if err != nil {
		ZapLog.Error("无法读取hosts文件",
			zap.Error(err),
		)
	}

	// 检查是否已存在条目
	entry := ip + "\t" + domain
	if strings.Contains(string(content), entry) {
		return
	}

	// 添加新条目
	content = append(content, []byte("\n"+entry)...)

	// 写入修改后的内容
	err = os.WriteFile(hostsFilePath, content, os.ModeAppend)
	if err != nil {
		ZapLog.Error("无法写入hosts文件",
			zap.Error(err),
		)
	}
	return
}

// CheckDefenderApp 检测是否安装了未兼容的软件
func CheckDefenderApp(products, defender *[]string) string {
	/*defender := []string{
		"火绒安全软件",
		"360安全卫士",
		"360杀毒",
		"电脑管家",
		"金山毒霸",
	}*/
	for _, v := range *products {
		for _, vv := range *defender {
			if strings.Contains(v, vv) {
				return v
			}
		}
	}
	return ""
}

// GetAllProduct 获取全部已安装的软件
func GetAllProduct() []string {
	softWares32, err := getProduct(registry.WOW64_32KEY)
	if err != nil {
		ZapLog.Error("读取已安装的32位软件失败",
			zap.Error(err),
		)
	}
	softWares64, err := getProduct(registry.WOW64_64KEY)
	if err != nil {
		ZapLog.Error("读取已安装的64位软件失败",
			zap.Error(err),
		)
	}
	var resultStrings []string
	UniqueAndMerge(softWares32, softWares64, &resultStrings, func(s string) string {
		return s
	})
	return resultStrings
}

// PreventSleepWindows 阻止休眠
func PreventSleepWindows() uintptr {
	return setThreadExecutionState(EsSystemRequired | EsDisplayRequired | EsContinuous)
}

// AllowSleepWindows 可以休眠
func AllowSleepWindows() {
	setThreadExecutionState(EsContinuous)
}

// SetWindowsTime 将时间设置到系统时间
func SetWindowsTime(ntpAddr string) bool {
	if ntpAddr == "" {
		ntpAddr = "ntp.aliyun.com"
	}
	// 获取网络时间
	ntpTime, err := ntp.Time(ntpAddr)
	if err != nil {
		ZapLog.Error("读取网络时间错误",
			zap.Error(err),
		)
		return false
	}
	ntpTime = ntpTime.UTC()
	// 构建windowsTime结构
	st := windowsTime{
		wYear:         uint16(ntpTime.Year()),
		wMonth:        uint16(ntpTime.Month()),
		wDayOfWeek:    uint16(ntpTime.Weekday()),
		wDay:          uint16(ntpTime.Day()),
		wHour:         uint16(ntpTime.Hour()),
		wMinute:       uint16(ntpTime.Minute()),
		wSecond:       uint16(ntpTime.Second()),
		wMilliseconds: uint16(ntpTime.Nanosecond() / 1000000),
	}
	// 调用SetSystemTime函数
	ret, _, err := setSystemTime.Call(
		uintptr(unsafe.Pointer(&st)),
	)
	if ret == 0 {
		ZapLog.Error("设置系统时间失败",
			zap.Error(err),
			zap.Time("network_time", ntpTime),
		)
	}
	ZapLog.Info("设置系统时间成功",
		zap.Time("network_time", ntpTime),
	)
	return true
}
