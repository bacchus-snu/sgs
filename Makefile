CSS_SRC := view/static/styles.css
CSS_DST := view/static/dist/styles.css

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
build:
	go build -o ./sgs ./cmd/sgs

.PHONY: clean
clean:
	rm -f $(CSS_DST)
	rm -f view/*_templ.go
	rm -f sgs

.PHONY: build-deps
build-deps:
	awk -F'"' '/\t/{print $$2}' tools.go \
		| xargs -t go install

.PHONY: check
check:
	SGS_TEST_DBURL="postgres://sgs:sgs-pass@localhost:5433/sgs?sslmode=disable" \
		go test $(TEST_ARGS) ./...
