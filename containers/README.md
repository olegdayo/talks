# Container internals or how 4 syscalls changed the operations

## Plan

- Introduction (2m)
- Main part (30m)
    - Example of a container and its features (5m)
        - Isolation of FS
        - Isolation of processes
        - Resource limits
        - Isolation of network
    - Let's implement our own container (25m)
        - The fork and exec syscalls (2m)
        - ChRoot (2m)
        - Setup bins and libs (1m)
        - Namespaces (5m)
        - CGroups (5m)
        - Networking theory (5m)
        - FS theory, mounts, security (5m)
- Conclusion (3m)

## LXC

### Launching the container

### Check, which features it has

#### Processes isolation

Check, that we don't have any processes of host in `ps aux`

#### Resource limiting

Check, that we can limit CPU, RAM, disk I/O, etc...

#### Filesystem isolation

Check, that we cannot access host systems files

#### Changing host info

Check, that we cannot get host info (or that it is changed at least)

## Launching your first container with zero abstractions

### Processes isolation

Let's take a look at `/proc`

We will use linux namespaces for it

```go
func setNamespace() error {
	return nil
}
```

### Resource limiting

We will use cgroups for it

```go
func setResourceLimits() error {
	return nil
}
```

### Filesystem isolation

```go
func changeDirectory(path string) error {
	err := syscall.Mkdir(path, 0o777)
	if err != nil {
		return err
	}

	err = syscall.Chroot(path)
	if err != nil {
		return err
	}

	// err = os.Chdir(path)
	// if err != nil {
	// 	return err
	// }

	return nil
}
```

### Add userspace

Due to new filesystem, we don't really have userspace now

We are going to do it manually

```go
// TODO
```

### Changing host info

```go
func updateHost() error {
	return nil
}
```

### Running requested binary

It will be similar to `docker -it`

We will use `exec` syscall to rewrite our processe's .text (executing code)

```go
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
```

## Conclusion

Container is just an isolated Linux process
