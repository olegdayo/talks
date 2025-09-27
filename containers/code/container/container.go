package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type executable struct {
	binary string
	libs   []string
}

func setupUserspace(path string) error {
	err := syscall.Mkdir(path, 0o777)
	if err != nil {
		fmt.Println(err)
	}

	binPath := filepath.Join(path, "bin")
	err = syscall.Mkdir(binPath, 0o777)
	if err != nil {
		fmt.Println(err)
	}

	libPath := filepath.Join(path, "lib")
	err = syscall.Mkdir(libPath, 0o777)
	if err != nil {
		fmt.Println(err)
	}

	lib64Path := filepath.Join(path, "lib64")
	err = syscall.Mkdir(lib64Path, 0o777)
	if err != nil {
		fmt.Println(err)
	}

	for _, executable := range []executable{
		{
			binary: "/usr/bin/sh",
			libs:   []string{""},
		},
		{
			binary: "/usr/bin/ls",
			libs:   []string{"/usr/lib/libcap.so.2", "/usr/lib/libc.so.6", "/usr/lib64/ld-linux-x86-64.so.2"},
		},
	} {
		cmd := exec.Command("cp", executable.binary, binPath)
		err = cmd.Run()
		if err != nil {
			return err
		}
		for _, lib := range executable.libs {
			targetLibPath := libPath
			if strings.Contains(lib, lib64Path) {
				targetLibPath = lib64Path
			}
			cmd := exec.Command("cp", lib, targetLibPath)
			err = cmd.Run()
			if err != nil {
				return err
			}
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
