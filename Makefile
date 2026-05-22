.PHONY: up down build

up:
	@test -f docker/.env || cp docker/.env.example docker/.env
	docker compose -f docker/docker-compose.yaml up --build -d

down:
	docker compose -f docker/docker-compose.yaml down

build:
	docker compose -f docker/docker-compose.yaml build
