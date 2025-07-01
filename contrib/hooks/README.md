# Git Hooks

This directory contains Git hooks that can be installed to help maintain code quality.

## Installation

To install the hooks, run:

```bash
cp contrib/hooks/* .git/hooks/
chmod +x .git/hooks/*
```

## Available Hooks

### post-commit
Automatically runs `go fmt ./...` after every commit to ensure all Go code is properly formatted.