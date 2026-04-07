default: build

build:
	go build -o terraform-provider-keel

install: build
	mkdir -p ~/.terraform.d/plugins/registry.terraform.io/keelapi/keel/0.1.0/$$(go env GOOS)_$$(go env GOARCH)
	mv terraform-provider-keel ~/.terraform.d/plugins/registry.terraform.io/keelapi/keel/0.1.0/$$(go env GOOS)_$$(go env GOARCH)/

test:
	go test ./... -v

testacc:
	TF_ACC=1 go test ./... -v

docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

.PHONY: default build install test testacc docs
