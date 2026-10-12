.PHONY: all build app run clean

all: app

# Build standalone CLI binary
build:
	go build -o lg-ac-menubar .

# Build and package into LG AC.app
app: build
	mkdir -p "LG AC.app/Contents/MacOS"
	cp lg-ac-menubar "LG AC.app/Contents/MacOS/lg-ac-menubar"

# Run the app
run: app
	open "LG AC.app"

# Clean built binaries
clean:
	rm -f lg-ac-menubar "LG AC.app/Contents/MacOS/lg-ac-menubar"
