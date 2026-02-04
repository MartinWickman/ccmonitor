ccmonitor:
	go build -ldflags="-s -w" -trimpath -o ccmonitor ./cmd/ccmonitor/

.PHONY: clean
clean:
	rm -f ccmonitor
