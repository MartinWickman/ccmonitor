ccmonitor:
	go build -ldflags="-s -w" -trimpath -o ccmonitor ./cmd/ccmonitor/

.PHONY: test
test:
	go test ./...

.PHONY: integration
integration: ccmonitor
	bash test-integration.sh

.PHONY: clean
clean:
	rm -f ccmonitor
