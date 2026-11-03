//go:build lite

package variant

const Name = "lite"
const DefaultKernel = "mihomo"

func Kernels() []string { return []string{"mihomo"} }
