# GitHub Actions Configuration

## Personal Access Token (PAT) Setup

This repository uses GitHub Actions to automatically build and push Docker images to GitHub Container Registry. To enable this functionality, you need to set up a Personal Access Token (PAT).

### Creating a PAT

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token (classic)"
3. Give your token a descriptive name (e.g., "HL7 Replicator Actions")
4. Set expiration as needed
5. Select the following scopes:
   - `write:packages` - Upload packages to GitHub Package Registry
   - `read:packages` - Download packages from GitHub Package Registry
   - `delete:packages` - Delete packages from GitHub Package Registry (optional)
   - `repo` - Full control of private repositories (if your repo is private)

### Adding PAT to Repository Secrets

1. Go to your repository settings
2. Navigate to Secrets and variables → Actions
3. Click "New repository secret"
4. Name: `PAT`
5. Value: Paste your Personal Access Token
6. Click "Add secret"

### Workflow Files

- `docker-publish.yml`: Builds and publishes Docker images to ghcr.io
- `test.yml`: Runs tests and linting

The workflows will automatically trigger on:
- Push to main/master branch
- Pull requests
- Version tags (v*)