# AGENTS.md

## Purpose
Instructions for coding agents working in this module.

## Scope
- Applies to files under this folder.
- Keep edits focused on the user request.

## Preferred workflow
1. Read relevant files before editing.
2. Make minimal, targeted changes.
3. Preserve current API behavior unless a breaking change is explicitly requested.
4. Run tests before finalizing.

## Testing policy
- Default expectation: run full module tests after code changes.
- Standard command:
  - `go test ./...`

## Safety rules
- Do not run destructive git commands unless explicitly requested.
  - Examples to avoid by default: `git reset --hard`, `git checkout --`, force pushes.
- Do not delete user data or files outside the requested scope.
- If unexpected unrelated changes are detected, stop and ask.

## Secrets and sensitive data
- Never print or expose secret values.
- Do not copy token contents from `.env` files into logs or responses.
- Redact sensitive values when showing command output.

## Notes for this module
- Keep provider behavior and tests deterministic where possible.
- Prefer adding or updating tests with behavior changes.
