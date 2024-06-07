CSS_SRC := view/static/styles.css
CSS_DST := view/static/dist/styles.css

BINS := sgs sgs-register-harbor

.PHONY: all
all: generate build

.PHONY: generate
generate: generate-templ generate-tailwindcss
.PHONY: generate-templ
generate-templ:
	templ generate
.PHONY: generate-tailwindcss
generate-tailwindcss:
	npx postcss $(CSS_SRC) -o $(CSS_DST)

.PHONY: build
build: $(BINS)

.PHONY: sgs
sgs:
	go build -o ./sgs ./cmd/sgs

.PHONY: sgs-register-harbor
sgs-register-harbor:
	go build -o ./sgs-register-harbor ./cmd/sgs-register-harbor

.PHONY: clean
clean:
	rm -f $(CSS_DST)
	rm -f view/*_templ.go
	rm -f $(BINS)

.PHONY: build-deps
build-deps:
	awk -F'"' '/\t/{print $$2}' tools.go \
		| xargs -t go install

.PHONY: check
check:
	SGS_TEST_DBURL="postgres://sgs:sgs-pass@localhost:5433/sgs?sslmode=disable" \
		go test $(TEST_ARGS) ./...
