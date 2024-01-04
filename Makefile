build_dir := build/
build:
	for tool in ./quoteadder/ ./watchAdir/ ./barker/ ./cw/ ./calshow/ ./describeFiles/ ./dripC/ ./filterMyCal/ ; do \
		go build -o ${build_dir} $$tool ; \
	done
