.PHONY: dev up down tidy web-install

dev:
	docker compose -f deploy/docker-compose.dev.yml up --build

up:
	docker compose -f deploy/docker-compose.yml up --build -d

down:
	docker compose -f deploy/docker-compose.yml down
	docker compose -f deploy/docker-compose.dev.yml down

tidy:
	cd api && go mod tidy

web-install:
	cd web && npm install
