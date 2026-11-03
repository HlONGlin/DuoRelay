//go:build lite

package configuration

import (
	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/generator/mihomo"
	"github.com/holi/duorelay/internal/ir"
)

func registeredGenerators() map[ir.KernelType]generator.Generator {
	return map[ir.KernelType]generator.Generator{ir.KernelMihomo: mihomo.NewGenerator()}
}
