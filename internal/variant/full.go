//go:build !lite

package variant

const Name = "full"
const DefaultKernel = "sing-box"

func Kernels() []string { return []string{"sing-box", "mihomo"} }
