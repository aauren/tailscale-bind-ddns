# Contributing

This repository welcomes contributions!

For small changes / bug fixes feel free to follow the steps outlined below.

For larger changes, please open an issue in this repository first so that it can be discussed with the maintainers first.

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass (`make ci`)
6. Submit a pull request

## Development Workflow

```bash
# Set up development environment
make deps

# Make your changes...

# Test the build
make tailscale-bind-ddns

# Run tests and linting
make ci

# If possible run GitHub actions locally to ensure they work correctly via act (https://github.com/nektos/act)
act
```
