MIGRATIONS_DIR = ./cmd/migrate/migrations

.PHONY: migration
migration:
	@migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migrate-up
migrate-up:
	@migrate -path $(MIGRATIONS_DIR) -database "$(DB_ADDR)" up

.PHONY: migrate-down
migrate-down:
	@migrate -path $(MIGRATIONS_DIR) -database "$(DB_ADDR)" down $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migrate-force
migrate-force:  
	@migrate -path=$(MIGRATIONS_DIR) -database="$(DB_ADDR)" force $(filter-out $@,$(MAKECMDGOALS))

.PHONY: run
run:
	go run ./cmd/api/

.PHONY: gen-docs
gen-docs:
	swag init -g cmd/api/main.go -d .

%:
	@: