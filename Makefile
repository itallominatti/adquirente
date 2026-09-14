DATABASE_URL ?= postgres://adq:adq@localhost:5432/adquirente?sslmode=disable

.PHONY: run test lint generate migrate-up migrate-down compose-up compose-down certs settle

run:            ## roda a API local (precisa de compose-up)
	go run ./cmd/api

test:           ## testes com detector de race
	go test -race -cover ./...

lint:
	go vet ./...
	golangci-lint run ./...
	govulncheck ./...

generate:       ## gera código gRPC a partir dos .proto
	buf lint && buf generate

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

compose-up:     ## sobe tudo (postgres, redis, kafka, jaeger, prometheus, grafana e os serviços)
	docker compose up -d --build

compose-down:
	docker compose down -v

certs:          ## chaves de dev: JWT (RSA) e um par de clientes; NUNCA use em produção
	./scripts/gen-dev-certs.sh

settle:         ## liquida a data informada: make settle DATE=2026-10-13
	SETTLEMENT_DATE=$(DATE) go run ./cmd/settlement
