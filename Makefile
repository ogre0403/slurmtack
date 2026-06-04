.PHONY: up down build sif

up:
	@test -f docker/.env || cp docker/.env.example docker/.env
	docker compose -f docker/docker-compose.yaml up --build -d

down:
	docker compose -f docker/docker-compose.yaml down

clean: down
	rm docker/*.db

build:
	docker compose -f docker/docker-compose.yaml build

sif:
	sudo ./build/build-placeholder-agent.sh
	@if [ -f build/output/placeholder-agent.sif ]; then \
		if [ -n "$$SUDO_UID" ] && [ -n "$$SUDO_GID" ]; then \
			sudo chown -R $$SUDO_UID:$$SUDO_GID build/output; \
		else \
			sudo chown -R $$(id -u):$$(id -g) build/output; \
		fi \
	fi
