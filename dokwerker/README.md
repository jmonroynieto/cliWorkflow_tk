# DockerHelper CLI

A streamlined Go CLI for managing containerized development environments across Go and TypeScript stacks. It automates boilerplate setup, handles container footprints, and maps host `UID`/`GID` to prevent file permission issues with bind mounts.

## Quick Start & Examples

```bash
# 1. Full workflow: Initialize, build, and jump into a TypeScript dev shell with a custom project name
dockerhelper fresh --stack ts --project bibtex-1

# 2. Just copy Go boilerplate files into the current directory
dockerhelper init --stack go

# 3. Build and launch your Go containers in the background
dockerhelper up --stack go

# 4. Open an interactive shell in a running TypeScript stack
dockerhelper shell --stack ts --project bibtex-1

# 5. Stop and clean up the project stack
dockerhelper down --project bibtex-1
