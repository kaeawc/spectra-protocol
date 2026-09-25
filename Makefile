.PHONY: fmt-check vet test deps-check ci

fmt-check:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }

vet:
	go vet ./...

test:
	go test -race -count=1 ./...

deps-check:
	@test "$$(go list -m all)" = "github.com/kaeawc/spectra-protocol" || { go list -m all; exit 1; }

ci: fmt-check vet test deps-check
