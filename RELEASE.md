# Release Process

This project uses [GoReleaser](https://goreleaser.com/) for automated releases and [GitHub Actions](https://github.com/features/actions) for CI/CD.

## Release Process

1. **Tag a release**: Create a git tag (e.g., `v1.0.0`)
2. **Push the tag**: `git push origin v1.0.0`
3. **Automated build**: GitHub Actions automatically builds binaries for multiple platforms
4. **Release creation**: GoReleaser creates a GitHub release with:
   - Pre-built binaries for Linux (amd64, arm64), macOS (amd64, arm64), and Windows (amd64, arm64)
   - Checksums for verification
   - Release notes with changelog

## Supported Platforms

- **Linux**: amd64, arm64
- **macOS**: amd64, arm64
- **Windows**: amd64, arm64

## CI/CD Pipeline

The project includes two GitHub Actions workflows:

- **CI** (`.github/workflows/ci.yml`): Runs on every push and PR
  - Tests the build process
  - Runs linter (golangci-lint)
  - Runs unit tests

- **Release** (`.github/workflows/release.yml`): Runs on tag pushes
  - Builds binaries for all supported platforms
  - Creates GitHub release with assets
  - Generates checksums for verification
