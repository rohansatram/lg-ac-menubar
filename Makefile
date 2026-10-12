.PHONY: all build app install uninstall release run clean

all: app

# Build standalone CLI binary
build:
	go build -o lg-ac-menubar .

# Build and package into LG AC.app bundle
app: build
	mkdir -p "LG AC.app/Contents/MacOS"
	cp lg-ac-menubar "LG AC.app/Contents/MacOS/lg-ac-menubar"

# Package zip for GitHub Release
release: app
	rm -f LG-AC-macOS-arm64.zip
	zip -r -y -q LG-AC-macOS-arm64.zip "LG AC.app"
	@echo "Created LG-AC-macOS-arm64.zip for GitHub Release"

# Install to macOS /Applications
install: app
	rm -rf "/Applications/LG AC.app"
	cp -r "LG AC.app" "/Applications/"
	@echo "Successfully installed to /Applications/LG AC.app"

# Uninstall from /Applications
uninstall:
	rm -rf "/Applications/LG AC.app"
	@echo "Removed /Applications/LG AC.app"

# Run the app directly
run: app
	open "LG AC.app"

# Clean built binaries
clean:
	rm -f lg-ac-menubar LG-AC-macOS-arm64.zip "LG AC.app/Contents/MacOS/lg-ac-menubar"
