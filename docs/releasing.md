# Releasing

Releases are created automatically from Git tags matching `v*`.

## Release outputs

For a tag such as `v1.2.3`, the workflow publishes:

```text
pxon_1.2.3_darwin_arm64.zip
pxon_1.2.3_darwin_amd64.zip
pxon_1.2.3_linux_arm64.tar.gz
pxon_1.2.3_linux_amd64.tar.gz
pxon_1.2.3_checksums.txt
```

The binaries report the release version and abbreviated commit through:

```sh
pxon --version
```

## Release process

1. Ensure `main` is clean and pushed.
2. Run the local tests:

   ```sh
   go test ./...
   go vet ./...
   ```

3. Create and push a semantic version tag:

   ```sh
   git tag v1.2.3
   git push origin v1.2.3
   ```

4. Wait for the `Release` GitHub Actions workflow.
5. Verify the GitHub release assets and checksums.
6. Verify the formula commit in `illegalstudio/homebrew-tap`.
7. Test the published package:

   ```sh
   brew update
   brew install illegalstudio/tap/pxon
   pxon --version
   ```

## Pipeline

The release workflow:

1. Runs unit tests and `go vet`.
2. Builds macOS and Linux binaries for ARM64 and AMD64 with GoReleaser.
3. Injects the tag version and commit into the binary.
4. Signs macOS binaries with the Apple Developer ID certificate.
5. Creates and publishes the GitHub release.
6. Notarizes the macOS archives and replaces the original release assets.
7. Recomputes the checksum file after notarization.
8. Generates `Formula/pxon.rb` from the final archive checksums.
9. Pushes the formula to `illegalstudio/homebrew-tap`.

The source-controlled formula generator is:

```text
scripts/generate-homebrew-formula.sh
```

## Organization secrets

The workflow expects these secrets to be available to the repository:

| Secret | Purpose |
|---|---|
| `APPLE_CERTIFICATE_P12` | Base64-encoded Apple Developer ID certificate. |
| `APPLE_CERTIFICATE_PASSWORD` | Password for the certificate archive. |
| `APPLE_SIGNING_IDENTITY` | Developer ID Application signing identity. |
| `APPLE_ID` | Apple ID used by `notarytool`. |
| `APPLE_TEAM_ID` | Apple Developer team identifier. |
| `APPLE_APP_PASSWORD` | App-specific password used for notarization. |
| `HOMEBREW_TAP_TOKEN` | Token allowed to update `illegalstudio/homebrew-tap`. |

`GITHUB_TOKEN` is provided automatically by GitHub Actions and is granted `contents: write` by the workflow.

## Local snapshot validation

With GoReleaser installed:

```sh
goreleaser check
goreleaser release --snapshot --clean --skip=publish
```

Without `APPLE_SIGNING_IDENTITY`, the local macOS signing hook reports that signing was skipped.
