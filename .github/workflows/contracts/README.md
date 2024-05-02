## Chainloop contracts


> **Disclaimer**: All contracts stored in the `.github/workflows/contracts` folder are a showcase of the Chainloop
internal build, package, release pipeline managed by the core maintainers of how to maintain contracts declaratively from a git repository. Real contracts should be stored
in a different repository and managed by the sec/ops team.

In this folder all contracts of Chainloop repository are kept as a reference and are the source of truth.

On every push to the `main` branch of the repository, all files contained in `.github/workflows/contracts`
are updated regardless if they have changed or not. No worries, Chainloop can detect if the file has changed or not
and update the revision accordingly.

Additionally, the contracts are being updated every day at midnight UTC.

All logic is behind handled by the GitHub action `.github/workflows/sync_contracts.yml`.

### Important note
The name of the files in `.github/workflows/contracts` should be the same as the contract name since
it's used for its update. Example:
- Contract name: `chainloop-deploy`
- File name: `chainloop-deploy.yml`

Generated update command:
```bash
$ chainloop wf contract update --name chainloop-deploy --contract .github/workflows/contracts/chainloop-deploy.yml
```