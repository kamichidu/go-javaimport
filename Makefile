TAGS=
VERBOSE=

.PHONY: test
test:
	go test ${VERBOSE} $$(glide novendor)

.PHONY: build
build:
	go build ${TAGS} -ldflags "-X main.appVersion=$$(git describe --tags)" .

.PHONY: debug
debug: build
	./go-javaimport -cp "${JAVA_HOME}\jre\lib\rt.jar"

.PHONY: deps
deps:
	go get -v github.com/Masterminds/glide
	go get -v golang.org/x/tools/cmd/benchcmp
	glide install
