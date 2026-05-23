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

generate-docs:
	cd tools && go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-dir=.. --provider-name=keel

docs: generate-docs

.PHONY: default build install test testacc generate-docs docs
