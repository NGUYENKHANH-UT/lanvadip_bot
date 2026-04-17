.PHONY: run
run:
	go run ./cmd/api/

.PHONY: gen-docs
gen-docs:
	swag init -g cmd/api/main.go -d .

%:
	@: