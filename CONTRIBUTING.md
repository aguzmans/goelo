# Contributing

Run the same checks used by GitHub Actions before opening a pull request:

```sh
go test ./...
go vet ./...
go build ./cmd/credrate
```

Rating changes must preserve or deliberately update the conformance vectors in `core/vectors/`.
Changes to a public Go API follow semantic versioning: additive changes are minor releases before
1.0.0; incompatible changes require a major release once the module reaches 1.0.0.

## Release versioning

Use [Conventional Commits](https://www.conventionalcommits.org/) so release-please can choose the
next semantic version:

- `fix:` requests a patch release.
- `feat:` requests a minor release.
- A `!` after the type or a `BREAKING CHANGE:` footer requests a major release.
- `docs:`, `test:`, `ci:`, and `chore:` do not request a release by themselves.

Merging regular changes to `main` runs CI and updates the release PR. Merging that release PR creates
the `vMAJOR.MINOR.PATCH` tag and GitHub release; GoReleaser attaches CLI archives and checksums.
