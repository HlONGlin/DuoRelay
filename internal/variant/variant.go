package variant

func SupportsKernel(name string) bool {
	for _, kernel := range Kernels() {
		if name == kernel {
			return true
		}
	}
	return false
}

func SupportsProtocol(protocol string) bool {
	if Name != "lite" {
		return true
	}
	switch protocol {
	case "shadowtls", "naive", "tor", "openconnect":
		return false
	}
	return true
}
