build:
	go build -v ./...

install: build
	go install -v ./...

generate:
	cd tools; go generate ./...

test:
	go test ./...

# Acceptance tests run against the Docker daemon from DOCKER_HOST (or the
# default socket). To use OpenTofu instead of Terraform:
#   TF_ACC_TERRAFORM_PATH=$(which tofu) TF_ACC_PROVIDER_HOST=registry.opentofu.org make testacc
testacc:
	TF_ACC=1 go test ./... -v -run TestAcc -timeout 10m
