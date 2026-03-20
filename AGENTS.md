# Agent Entry

## First Read
- `docs/product.md`
- `docs/architecture.md`
- `plans/`
- `DEV_GUIDE.md`

## Rules
- Do not bypass documented architecture boundaries.
- Do not hide provider-specific transport quirks in unrelated business logic.
- Treat embedded prompt assets as product code.
- Keep changes minimal unless the active plan explicitly calls for refactoring.
- Run `bash scripts/validate.sh` before finishing meaningful implementation work.

## Task Flow
1. Read the active plan in `plans/`
2. Map each AC to implementation and validation
3. Implement the smallest change that satisfies the plan
4. Update docs/examples if behavior changes
5. Run validation
6. Fix failures before handing off
