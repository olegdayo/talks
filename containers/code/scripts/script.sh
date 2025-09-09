host:~$ sudo docker run --rm -it --entrypoint /bin/sh alpine
cont:/$ echo hello
hello

host:~$ sudo docker run --rm -it --entrypoint /bin/sh alpine

host:~$ ps aux | grep script.sh | grep -v grep
root      335720 92.5 ... /bin/sh ./script.sh
