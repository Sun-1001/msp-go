# Backend Go Migration Cleanup Audit

> Archive note: this is a point-in-time cleanup audit. Current work is tracked in [项目待办](../../TODO.md); current technical behavior is documented under `docs/technical/`.

**Audit date**: 2026-05-07  
**Capability recheck**: 2026-07-10
**Status**: CLEANUP COMPLETE, USER-OWNED RUNTIME VALIDATION

**Scope**: migrate the default backend runtime to Go, exclude legacy Python AI/Agent/OCR/LLM/math-solver implementation, keep incomplete Go AI capabilities behind explicit boundaries, and remove the legacy Python backend directory.

## Decision

The earlier audit kept `backend/` as a P8 comparison artifact because Python/Go double-run, exact error parity, nested DTO parity, browser smoke, Docker/Compose smoke, and performance baseline were not complete.

On 2026-05-07 the user explicitly decided to waive double-run/comparison in this workspace: "不用双跑，不用比对，我自己会测试，所以清理一下." The user then confirmed deleting `backend/`, including ignored `.venv`, cache, and old `uploads`.

## Cleanup Evidence

| Requirement | Evidence | Status |
|---|---|---:|
| Go is the default backend runtime | `start.bat`, `docker-compose.yml`, `backend-go/Dockerfile`, `frontend/nginx.conf`, `nginx-site.conf`, `scripts/deploy.sh`, `scripts/update.sh` point to Go backend / `msp-migrate` | PASS |
| Legacy Python backend removed | `rm -rf backend`; `test -e backend; echo $?` returned `1` | PASS |
| AI boundary and implemented slices explicitly guarded | `backend-go/tests/contract/ai_boundary_surface_test.go` guards the legacy stack exclusion and dedicated `OCR_UNAVAILABLE` boundary; exercise application/HTTP tests cover transaction-free image-only rejection | PASS |
| Frontend calls statically covered or classified | `backend-go/tests/contract/frontend_route_surface_test.go` | PASS |
| Runtime double-run/comparison | User explicitly owns testing and waived blocking comparison for cleanup | USER-OWNED |
| Docker/Compose/browser/performance smoke | User explicitly owns runtime testing | USER-OWNED |

## 2026-07-10 Capability Recheck

The AI-adjacent Go routes are no longer accurately described as blanket TODO/placeholders. Admin provider/model/Agent configuration and the Eino Tutor, Portrait, Diagnostician, Math Solver, and Question Parser slices have executable implementations with documented local fallbacks. P6 remains `IN_PROGRESS`; this recheck does not claim full AI workflow completion.

`POST /api/v1/exercise/submit` supports text grading and diagnosis. Text accompanied by an image URL is still graded from the text. Because OCR is not implemented, an image-only answer now fails closed with HTTP `501` and code `OCR_UNAVAILABLE` before opening a transaction. It creates no attempt, diagnosis, learning session, or DKT update. The frontend no longer exposes the non-functional handwritten-answer upload path.

## Residual Risk

Because `backend/` is deleted, this workspace can no longer run Python/Go double-run or inspect the legacy Python implementation without restoring it from git history or an external archive.

Runtime API behavior, Docker/Compose smoke, browser/API flow smoke, performance baseline, and external live integrations are intentionally left to user validation per the user's instruction.

The remaining work recorded at this point in time has since been consolidated into the current TODO and technical documents.
