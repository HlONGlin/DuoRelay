//go:build singbox && !lite && !mihomo

package variant

const Name = "singbox"
const DefaultKernel = "sing-box"

func Kernels() []string { return []string{"sing-box"} }
