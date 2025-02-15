GIT_TAG := $(shell git rev-parse --short HEAD)
.PHONY: build
build_dir := build/
build:
	for tool in ansCRUBi/ barker/ calshow/ chaptor/ cw/ describeFiles/ dripC/ fickleFinger/ filterMyCal/ indexFiles/ kompti/ mdMake/ quoteadder/ watchAdir/ xwin/ zustellen/; do \
	go build --ldflags="-X main.CommitId=$(GIT_TAG)" -o ${build_dir} ./$$tool ; \
	done
