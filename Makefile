DOCKER_IMAGE=hawk.catcher

docker: docker-build docker-run

docker-build:
	docker build -t $(DOCKER_IMAGE) -f Dockerfile .
docker-run:
	docker-compose up