# Repository Guidelines

This `AGENTS.md` file defines the directives that apply to the entire `SkyChain` repository. Follow these instructions for any files you touch within this repository.

## Scope
- These rules apply to every directory and file in this repository unless a more specific `AGENTS.md` is introduced in a subdirectory.

## General Coding Requirements
1. All production code must be written in Go.
2. Place Go packages inside the `/pkg` directory.
3. The main executable entry point belongs under `/cmd/skychain`.
4. All public functions must include comments explaining their purpose, parameters, and return values.

## Documentation and Contribution Notes
- When updating or creating files, ensure they comply with Go conventions and the structure outlined above.
- If you add subdirectories that need their own guidance, include a nested `AGENTS.md` file with the additional rules.
- Keep this document up to date if repository-wide conventions change.
- Major implementation changes should be reflected in `README.md` to keep it current with key features and usage instructions.
- If a feature requires a configuration file, a well-documented sample should be added to a `/hack` directory.

## PR and Testing Expectations
- Provide meaningful commit messages describing the changes you introduce.
- Before submitting a pull request, run the appropriate Go formatting and testing commands relevant to your changes.
- Every piece of production code must be accompanied by Go tests that live in the same package as the code they verify. Strive for 100% coverage whenever practical and justify any intentional gaps.

Adhering to these guidelines helps maintain consistency across the project and streamlines collaboration.
