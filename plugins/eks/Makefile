BINARY := marmot-plugin-eks
# The directory Marmot scans for local plugins.
MARMOT_PLUGINS_DIR ?= $(HOME)/.marmot/plugins

.PHONY: build test install clean

build:
	go build -o bin/$(BINARY) .

test:
	go test ./...

install: build
	mkdir -p $(MARMOT_PLUGINS_DIR)
	cp bin/$(BINARY) $(MARMOT_PLUGINS_DIR)/$(BINARY)

clean:
	rm -rf bin
