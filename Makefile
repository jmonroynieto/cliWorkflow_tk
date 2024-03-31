build_dir := build/
build:
	for tool in ./kompti ./chaptor/ ./quoteadder/ ./watchAdir/ ./barker/ ./cw/ ./calshow/ ./describeFiles/ ./dripC/ ./filterMyCal/ ./ansCRUBi/ ; do \
		go build -o ${build_dir} $$tool ; \
	done
