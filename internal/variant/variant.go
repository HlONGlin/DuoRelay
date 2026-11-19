package variant

import (
	"runtime"

	"github.com/holi/duorelay/internal/ir"
)

func Enabled() bool    { return len(Kernels()) > 1 }
func Platform() string { return runtime.GOOS + "/" + runtime.GOARCH }

func SupportsKernel(name string) bool {
	for _, kernel := range Kernels() {
		if name == kernel {
			return true
		}
	}
	return false
}

func SupportsProtocol(protocol string) bool {
	switch protocol {
	case ir.ProtocolProxyChain:
		return true
	case ir.ProtocolLoadBalance:
		return SupportsKernel("mihomo")
	}
	owner := ir.GetProtocolOwner(protocol)
	return (SupportsKernel("mihomo") && owner&ir.KernelSetMihomo != 0) ||
		(SupportsKernel("sing-box") && owner&ir.KernelSetSingBox != 0)
}
