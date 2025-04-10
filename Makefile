GIT_TAG := $(shell git rev-parse --short HEAD)
.PHONY: build
build_dir := build/
build:
		@mkdir -p $(build_dir)
		@for tool in ansCRUBi ansible barker calshow chaptor cw describeFiles dripC fickleFinger filterMyCal indexFiles kompti kwiqExt mdMake megalophobia quoteadder watchAdir xwin zustellen; do \
			go build --ldflags="-X main.CommitId=$(GIT_TAG) -X main.Version=1.3 -s -w" -o $(build_dir)$$tool ./$$tool ; \
		done

install_dir := /home/pollo/Local/bin/

.PHONY: install
install: build
		@mkdir -p $(install_dir)
		@for tool in ansCRUBi ansible barker calshow chaptor cw describeFiles dripC fickleFinger filterMyCal indexFiles kompti kwiqExt mdMake megalophobia quoteadder watchAdir xwin zustellen; do \
			install -D $(build_dir)$$tool $(install_dir); \
		done
		@rm -d ${build_dir}
