# GHCR images

The **Publish GHCR image** GitHub Actions workflow builds the UI and Go application and publishes `linux/amd64` and `linux/arm64` images from `main`.

For this repository:

```sh
docker pull ghcr.io/jiayi-1994/thunderdome-planning-poker:latest
```

Each successful run also publishes `:main` and `:sha-<full-commit-SHA>`. Use the commit tag or the digest from the workflow summary to deploy a specific build.

Push to `main` to publish automatically, or choose **Actions → Publish GHCR image → Run workflow → main**. The workflow uses `GITHUB_TOKEN` with `packages: write`; no Docker Hub credentials or additional repository secrets are required. It verifies both image architectures and runs the AMD64 binary with `--version` after publishing.

GHCR package visibility is managed independently of repository visibility. For a private package, authenticate with an account that has package read access before pulling:

```sh
gh auth token | docker login ghcr.io -u "$(gh api user --jq .login)" --password-stdin
```

The image runs the existing `serve` command as an unprivileged user, listens on port 8080, and requires the application's PostgreSQL and other runtime settings. See `docker-compose.yml` for the existing configuration.

References: [GitHub package publishing](https://docs.github.com/en/actions/tutorials/publish-packages/publish-docker-images), [Docker cross-compilation](https://docs.docker.com/build/building/multi-platform/#cross-compilation).
