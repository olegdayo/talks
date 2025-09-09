package main

import (
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command("/bin/echo", "i <3 exec")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}
