GIT_TAG := $(shell git rev-parse --short HEAD)
.PHONY: build
build_dir := build/
build:
	for tool in kompti/ chaptor/ quoteadder/ watchAdir/ barker/ cw/ calshow/ describeFiles/ dripC/ filterMyCal/ ansCRUBi/ indexFiles/ ; do \
	go build --ldflags="-X main.CommitId=$(GIT_TAG)" -o ${build_dir} ./$$tool ; \
	done
