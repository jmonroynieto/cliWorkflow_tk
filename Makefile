GIT_TAG := $(shell git rev-parse --short HEAD)
build_dir := build/
TOOLS := ansCRUBi ansible barker calshow cedula chaptor cueLine cw describeFiles dokwerker dripC fickleFinger gromula filterMyCal indexFiles janus kompti kwiqExt lineExplorer  lotajxo mdMake megalophobia quoteadder sdl shFossils talaria/cmd/talaria talaria/cmd/talariad watchAdir xwin zustellen
install_dir := /home/pollo/Local/bin/

.PHONY: build
build:
		@echo "current GIT_TAG is $(GIT_TAG)" 
		@mkdir -p $(build_dir)
		@for tool in $(TOOLS); do\
			name=$$(basename $$tool); \
			echo "--- Building $$name ---" ;\
			go build --ldflags="-X main.CommitId=$(GIT_TAG) -X main.Version=1.7 -s -w" -o $(build_dir)$$name ./$$tool ; \
		done

install_dir := /home/pollo/Local/bin/

.PHONY: install
install:
		@mkdir -p $(install_dir)
		@for tool in $(TOOLS) ; do \
			name=$$(basename $$tool) ; \
			echo "--- Installing $$name ---" ;\
			install -p $(build_dir)$$name $(install_dir)/$$name && rm $(build_dir)$$name || echo "=== Failed installing $$name ===" ; \
		done
		@rm -d ${build_dir}


.PHONY: install-systemd                                                                                                      
install-systemd:                                                                                                             
	install -d $(UNITDIR)                                                                                                    
	install -m 644 talaria/tools/systemd-services/talaria-maintain.service $(UNITDIR)/                                       
	install -m 644 talaria/tools/systemd-services/talaria-maintain.timer $(UNITDIR)/                                         
	@echo "systemctl --user daemon-reload"                                                                                   
	@echo "systemctl --user enable --now talaria-maintain.timer"
