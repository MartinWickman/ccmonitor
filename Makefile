ccmonitor:
	go build -ldflags="-s -w" -trimpath -o ccmonitor ./cmd/ccmonitor/

ccmonitor.exe:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o ccmonitor.exe ./cmd/ccmonitor/

.PHONY: test
test:
	go test ./...

.PHONY: integration
integration: ccmonitor
	bash test-integration.sh

.PHONY: clean
clean:
	rm -f ccmonitor
