.PHONY: bin
bin:
	mkdir -p bin 2> /dev/null
	go build -o bin container/*.go

.PHONY: clean
clean:
	rm -rf bin
	rm -rf /tmp/container
