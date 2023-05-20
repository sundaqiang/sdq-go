package sundry

import (
	"go.uber.org/zap"
	"golang.org/x/sys/windows/registry"
	"os"
	"strings"
	"syscall"
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	setThreadExecutionStateProc = kernel32.NewProc("SetThreadExecutionState")
)

const (
	EsContinuous      = 0x80000000
	EsSystemRequired  = 0x00000001
	EsDisplayRequired = 0x00000002
)

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

// AddHosts 添加hosts
func AddHosts(ip string, domain string) {
	hostsFilePath := "C:\\Windows\\System32\\drivers\\etc\\hosts"
	// 读取hosts文件
	content, err := os.ReadFile(hostsFilePath)
	if err != nil {
		zapLog.Error("无法读取hosts文件",
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
		zapLog.Error("无法写入hosts文件",
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
		zapLog.Error("读取已安装的32位软件失败",
			zap.Error(err),
		)
	}
	softWares64, err := getProduct(registry.WOW64_64KEY)
	if err != nil {
		zapLog.Error("读取已安装的64位软件失败",
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
