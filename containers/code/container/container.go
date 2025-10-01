package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func setupUserspace(path string) error {
	fmt.Println("setting up userspace")

	err := syscall.Mkdir(path, 0o777)
	if err != nil {
		fmt.Println("mkdir:", err)
	}

	for _, deps := range []string{"/usr", "/lib", "/lib64", "./scripts/benchmark.sh"} {
		cmd := exec.Command("cp", "-r", deps, path)
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func changeDirectory(path string) error {
	fmt.Println("changing directory")

	err := syscall.Chdir(path)
	if err != nil {
		return err
	}

	err = syscall.Chroot(path)
	if err != nil {
		return err
	}

	err = os.Setenv("PATH", "/usr/bin")
	if err != nil {
		return err
	}
	return nil
}

func setNamespace(cmd *exec.Cmd) error {
	fmt.Println("setting namespace")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID,
	}
	return nil
}

func setResourceLimits() error {
	fmt.Println("setting resource limits")

	err := os.MkdirAll("/sys/fs/cgroup", 0777)
	if err != nil {
		return err
	}

	err = syscall.Mount("/sys/fs/cgroup", "/sys/fs/cgroup", "cgroup2", 0, "")
	if err != nil {
		return err
	}

	path := "/sys/fs/cgroup/container"
	err = os.MkdirAll(path, 0777)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(path, "cpu.max"), []byte("50000 100000"), 0777)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(path, "cgroup.procs"), []byte(fmt.Sprintf("%d", os.Getpid())), 0777)
	if err != nil {
		return err
	}

	return nil
}

func run(argv []string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := setNamespace(cmd)
	if err != nil {
		return err
	}

	err = cmd.Run()
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
		checkErr(setupUserspace("/tmp/container"))
		checkErr(changeDirectory("/tmp/container"))
		checkErr(setResourceLimits())
		checkErr(run(argv[2:]))
	}
}
