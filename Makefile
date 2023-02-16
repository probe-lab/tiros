REPO_SERVER=019120760881.dkr.ecr.us-east-1.amazonaws.com

docker:
	$(eval GIT_TAG := $(shell git rev-parse --short HEAD))
	docker build -t "${REPO_SERVER}/probelab:tiros-${GIT_TAG}" .

docker-push: docker
	docker push "${REPO_SERVER}/probelab:tiros-${GIT_TAG}"

nodeagent:
	GOARCH=amd64 GOOS=linux GOBIN="$(PWD)" go install github.com/guseggert/clustertest/cmd/agent@latest
	mv agent nodeagent

tools:
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.15.2
	go install github.com/volatiletech/sqlboiler/v4@v4.14.1
	go install github.com/volatiletech/sqlboiler/v4/drivers/sqlboiler-psql@v4.14.1

database:
	docker run --rm -p 5439:5432 -e POSTGRES_PASSWORD=password -e POSTGRES_USER=tiros_test -e POSTGRES_DB=tiros_test postgres:14

models:
	sqlboiler --no-tests psql

migrate-up:
	migrate -database 'postgres://tiros_test:password@localhost:5439/tiros_test?sslmode=disable' -path migrations up

migrate-down:
	migrate -database 'postgres://tiros_test:password@localhost:5439/tiros_test?sslmode=disable' -path migrations down

.PHONY: nodeagent tools docker-push
