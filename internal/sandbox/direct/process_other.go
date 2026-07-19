//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package direct

import (
	"os/exec"
	"time"
)

const directWaitDelay = 2 * time.Second

func configureCommand(cmd *exec.Cmd) {
	cmd.WaitDelay = directWaitDelay
}
