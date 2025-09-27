package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func setupUserspace(path string) error {
	err := syscall.Mkdir(path, 0o777)
	if err != nil {
		fmt.Println(err)
	}

	for _, sysDir := range []string{"/usr", "/lib", "/lib64"} {
		cmd := exec.Command("cp", "-r", sysDir, path)
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	err = os.Setenv("PATH", "/usr/bin")
	if err != nil {
		return err
	}

	return nil
}

func changeDirectory(path string) error {
	err := syscall.Chroot(path)
	if err != nil {
		return err
	}
	return nil
}

func setNamespace() error {
	return nil
}

func setResourceLimits() error {
	return nil
}

func updateHost() error {
	return nil
}

func run(argv []string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	argv := os.Args
	fmt.Println(argv)
	switch argv[1] {
	case "run":
		setupUserspace("/tmp/container")
		checkErr(changeDirectory("/tmp/container"))
		checkErr(run(argv[2:]))
	}
}
