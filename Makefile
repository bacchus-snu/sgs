CSS_SRC := view/static/styles.css
CSS_DST := view/static/dist/styles.css

BINS := sgs sgs-register-harbor sgs-runc-wrapper

.PHONY: all
all: generate build

.PHONY: generate generate-templ generate-tailwindcss
generate: generate-templ generate-tailwindcss
generate-templ:
	templ generate
generate-tailwindcss:
	npx postcss $(CSS_SRC) -o $(CSS_DST)

.PHONY: build $(BINS)
build: $(BINS)
sgs:
	go build -o ./sgs ./cmd/sgs
sgs-register-harbor:
	go build -o ./sgs-register-harbor ./cmd/sgs-register-harbor
sgs-runc-wrapper:
	CGO_ENABLED=0 go build -o ./sgs-runc-wrapper ./cmd/sgs-containerd-shim

.PHONY: hotreload
hotreload:
	env $$(<.env.development xargs) air

.PHONY: serve-docs serve-docs-en serve-docs-ko
serve-docs: serve-docs-en
serve-docs-en:
	mdbook serve docs/en
serve-docs-ko:
	mdbook serve docs/ko

.PHONY: check
check:
	SGS_TEST_DBURL="postgres://sgs:sgs-pass@localhost:5433/sgs?sslmode=disable" \
		go test $(TEST_ARGS) ./...

.PHONY: clean
clean:
	rm -f $(CSS_DST)
	rm -f view/*_templ.go
	rm -f $(BINS)
