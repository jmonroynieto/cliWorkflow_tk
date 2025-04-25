GIT_TAG := $(shell git rev-parse --short HEAD)
build_dir := build/
TOOLS := ansCRUBi ansible barker calshow chaptor cw describeFiles dripC fickleFinger filterMyCal indexFiles kompti kwiqExt mdMake megalophobia quoteadder watchAdir xwin zustellen
install_dir := /home/pollo/Local/bin/

.PHONY: build
build:
		@echo "current GIT_TAG is $(GIT_TAG)" 
		@mkdir -p $(build_dir)
		@for tool in $(TOOLS); do\
			echo "--- Building $$tool ---" ;\
			go build --ldflags="-X main.CommitId=$(GIT_TAG) -X main.Version=1.3 -s -w" -o $(build_dir)$$tool ./$$tool ; \
		done

install_dir := /home/pollo/Local/bin/

.PHONY: install
install:
		@mkdir -p $(install_dir)
		@for tool in ansCRUBi ansible barker calshow chaptor cw describeFiles dripC fickleFinger filterMyCal indexFiles kompti kwiqExt mdMake megalophobia quoteadder watchAdir xwin zustellen; do \
			echo "--- Installing $$tool ---" ;\
			install -p $(build_dir)$$tool $(install_dir)/$$tool; \
		done
		@rm -r ${build_dir}
