GIT_TAG := $(shell git rev-parse --short HEAD)
.PHONY: build
build_dir := build/
build:
	for tool in kompti/ chaptor/ quoteadder/ watchAdir/ barker/ cw/ calshow/ describeFiles/ dripC/ filterMyCal/ ansCRUBi/ indexFiles/ ; do \
	echo "go build --ldflags=\"-X github.com/jmonroynieto/cliWorkflow_tk/$${tool}main.CommitId=$(GIT_TAG)\" -o ${build_dir} ./$$tool" ; \
	done
