test:
	CGO_CXXFLAGS="-I$$(brew --prefix leptonica)/include -I$$(brew --prefix tesseract)/include" \
	CGO_LDFLAGS="-L$$(brew --prefix leptonica)/lib -L$$(brew --prefix tesseract)/lib" \
	go test -race ./...