# daisctl

Welcome to **daisctl**! This is a command-line tool built with Go and Cobra that provides various utilities for managing and interacting with our platform.

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Commands](#commands)
- [Contributing](#contributing)
- [License](#license)

## Installation

To install daisctl, you can download the latest release from the [releases page](https://github.com/Altinn/altinn-platform/releases) or build it from source.

### Building from Source

1. **Clone the repository**:
   ```sh
   git clone https://github.com/altinn/altinn-platform.git
   cd altinn-platform/tools/daisctl
   ```

2. **Build the application**:
   ```sh
   make build
   ```

3. **Add the executable to your PATH** (optional):
   - On **Windows**:
     Add it to Environment variables in system properties
   - On **macOS/Linux**:
     ```sh
     export PATH=$PATH:/path/to/daisctl
     ```

## Usage

After installation, you can use daisctl by running the `dais` command followed by a specific command and options.

```sh
dais [command] [flags]
```

### Example

To check the version of daisctl:

```sh
dais version
Dais version: dais-v0.0.1
Commit: f6e5cacf1029e28a260b4a28fffee85eb4f67aa9
Build Date: 2024-07-30T10:58:16Z
```

## Commands

The CLI Application currently supports the following commands:

- **version**: Displays the current version of daisctl.
  ```sh
  dais version
  ```

- **releases**: Lists the current releases running.
  ```sh
  dais releases --app my-app
  dais r
  dais rel --app=myapp
  ```

- **help**: Displays help information for daisctl and its commands. 
  ```sh
  dais help
  ```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
