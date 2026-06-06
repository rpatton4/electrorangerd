.PHONY: all fmt lint build bundle clean-bundle reset-vault

BINARY := electrorangerd
BIN_DIR := bin

APP_NAME    := ElectroRangerD
APP_BUNDLE  := $(BIN_DIR)/$(APP_NAME).app
ICON_SRC    := packaging/macos/icon.png
ICON_ISET   := $(BIN_DIR)/icon.iconset
ICON_ICNS   := $(BIN_DIR)/icon.icns
INFO_PLIST  := packaging/macos/Info.plist

all: fmt lint build

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) ./cmd/electrorangerd

bundle: build $(ICON_ICNS) $(INFO_PLIST)
	@rm -rf $(APP_BUNDLE)
	@mkdir -p $(APP_BUNDLE)/Contents/MacOS
	@mkdir -p $(APP_BUNDLE)/Contents/Resources
	@cp $(INFO_PLIST) $(APP_BUNDLE)/Contents/Info.plist
	@cp $(BIN_DIR)/$(BINARY) $(APP_BUNDLE)/Contents/MacOS/$(BINARY)
	@cp $(ICON_ICNS) $(APP_BUNDLE)/Contents/Resources/icon.icns
	@echo "Bundled $(APP_BUNDLE)"

$(ICON_ISET): $(ICON_SRC) | $(BIN_DIR)
	@rm -rf $@
	@mkdir -p $@
	@sips -z 16   16   $< --out $@/icon_16x16.png       >/dev/null
	@sips -z 32   32   $< --out $@/icon_16x16@2x.png    >/dev/null
	@sips -z 32   32   $< --out $@/icon_32x32.png       >/dev/null
	@sips -z 64   64   $< --out $@/icon_32x32@2x.png    >/dev/null
	@sips -z 128  128  $< --out $@/icon_128x128.png     >/dev/null
	@sips -z 256  256  $< --out $@/icon_128x128@2x.png  >/dev/null
	@sips -z 256  256  $< --out $@/icon_256x256.png     >/dev/null
	@sips -z 512  512  $< --out $@/icon_256x256@2x.png  >/dev/null
	@sips -z 512  512  $< --out $@/icon_512x512.png     >/dev/null
	@sips -z 1024 1024 $< --out $@/icon_512x512@2x.png  >/dev/null

$(ICON_ICNS): $(ICON_ISET)
	@iconutil -c icns -o $@ $<

$(BIN_DIR):
	@mkdir -p $@

clean-bundle:
	@rm -rf $(APP_BUNDLE) $(ICON_ISET) $(ICON_ICNS)

# Destructive: wipes the encrypted vault blob and any stored OS-keychain
# DEK so the next launch lands on the "Set master password" first-run flow.
# Does not touch config.json (theme preference is unrelated to login state).
reset-vault:
	@rm -f "$$HOME/Library/Application Support/electrorangerd/vault.json"
	@security delete-generic-password -s ElectroRangerD -a vault-dek 2>/dev/null || true
	@echo "Vault state reset — next launch will prompt 'Set master password'."
