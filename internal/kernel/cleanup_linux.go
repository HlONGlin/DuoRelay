package kernel

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// cleanupMihomoTUNNetwork removes the policy-routing objects created by
// mihomo's Linux auto-redirect mode. mihomo does not reliably remove them
// when its managed process is stopped, so DuoRelay cleans only the documented
// priorities/table owned by this generated TUN configuration.
func cleanupMihomoTUNNetwork() error {
	var errs []string
	for _, priority := range []string{"9000", "9001", "9002", "9010"} {
		for {
			err := exec.Command("ip", "rule", "del", "priority", priority).Run()
			if err == nil {
				continue
			}
			break
		}
	}
	if output, err := exec.Command("ip", "route", "flush", "table", "2022").CombinedOutput(); err != nil && !isNetworkObjectMissing(output) {
		errs = append(errs, fmt.Sprintf("ip route flush table 2022: %v: %s", err, strings.TrimSpace(string(output))))
	}
	if output, err := exec.Command("nft", "delete", "table", "inet", "mihomo").CombinedOutput(); err != nil && !isNetworkObjectMissing(output) {
		errs = append(errs, fmt.Sprintf("nft delete table inet mihomo: %v: %s", err, strings.TrimSpace(string(output))))
	}
	if _, err := exec.Command("ip", "link", "show", "Meta").CombinedOutput(); err == nil {
		if output, deleteErr := exec.Command("ip", "link", "delete", "Meta").CombinedOutput(); deleteErr != nil && !isNetworkObjectMissing(output) {
			errs = append(errs, fmt.Sprintf("ip link delete Meta: %v: %s", deleteErr, strings.TrimSpace(string(output))))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func isNetworkObjectMissing(output []byte) bool {
	message := strings.ToLower(string(output))
	return strings.Contains(message, "no such process") ||
		strings.Contains(message, "no such file") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "not found")
}
