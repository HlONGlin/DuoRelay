//go:build mihomo && !lite && !singbox

package variant

const Name = "mihomo"
const DefaultKernel = "mihomo"

func Kernels() []string { return []string{"mihomo"} }
