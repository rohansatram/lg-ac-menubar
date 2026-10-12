# LG AC Menu Bar (macOS) ❄️

A super-lightweight, native macOS menu bar app to control **LG ThinQ Air Conditioners**, written in **Go**.

![macOS](https://img.shields.io/badge/macOS-Apple_Silicon_%7C_Intel-black?logo=apple)
![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)
![Binary Size](https://img.shields.io/badge/Binary_Size-8.5_MB-green)
![RAM](https://img.shields.io/badge/RAM_Usage-~20_MB-blue)
![License](https://img.shields.io/badge/License-MIT-purple)

---

## Highlights

- ⚡ **Zero Electron / Zero Node:** Pure Go with native macOS Cocoa & WebKit bindings via CGO. Compiles to an **8.5 MB static binary** and sips **~20 MB RAM**.
- ❄️ **Automatic Device Discovery:** Detects all air conditioners on your LG account with parallel state fetching via goroutines.
- 🟢 **Live Online/Offline Detection:** Disconnected or unpowered units are cleanly badged with disabled controls so you never trigger API conflicts.
- ⏻ **Instant Power Control:** One-click toggle with in-flight request protection.
- 🌡️ **Debounced Temperature Stepper:** Instant UI response when adjusting `[−]` and `[+]`, automatically debounced to prevent API queue collisions.
- 🏠 **Ambient Room Temperature:** Displays live room temperature readings alongside target temps.
- 🔑 **Simple In-App Onboarding:** Built-in first-run screen with one-click link to LG's official Personal Access Token portal and clipboard paste.

---

## Prerequisites

- macOS 12.0+ (Apple Silicon or Intel)
- One or more LG ThinQ air conditioners
- An LG Personal Access Token (PAT) from [connect-pat.lgthinq.com](https://connect-pat.lgthinq.com)

---

## Installation

### Option 1: Download Pre-Built App (Recommended)

1. Download the latest **`LG-AC-macOS-arm64.zip`** from [Releases](https://github.com/rohansatram/lg-ac-menubar/releases).
2. Unzip the file and move **`LG AC.app`** into your `/Applications` folder.
3. Open **LG AC** from Spotlight (`Cmd + Space`) or Launchpad.

> **Note on macOS Gatekeeper:** Because this open-source build is not signed with an Apple Developer ID certificate, macOS might show a warning on first open. Simply **Right-click (Control-click) `LG AC.app` → Open → Click Open**, or run:
> ```bash
> xattr -cr "/Applications/LG AC.app"
> ```

---

### Option 2: Build from Source

```bash
git clone https://github.com/rohansatram/lg-ac-menubar.git
cd lg-ac-menubar

# Build and install to /Applications
make install

# Launch
open "/Applications/LG AC.app"
```

### 2. First-Time Setup

1. Click the **❄️** icon in your macOS menu bar.
2. Click **Open LG Token Portal ↗** to log into LG and generate your Personal Access Token (PAT).
3. Click **📋 Paste** in the app and hit **Connect & Discover ACs**.
4. Your token is stored securely in `~/.config/lg-ac-menubar/config.json`.

---

## Development

```bash
# Build standalone binary
go build -o lg-ac-menubar .

# Run standalone
./lg-ac-menubar
```

---

## License

[MIT](LICENSE)
