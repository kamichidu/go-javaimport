TAGS=

build:
	go build ${TAGS} -ldflags "-X main.appVersion=$$(git describe --tags)" .

debug: build
	./go-javaimport -cp "${JAVA_HOME}\jre\lib\rt.jar"

deps:
	go get -v golang.org/x/tools/cmd/benchcmp
	glide install
