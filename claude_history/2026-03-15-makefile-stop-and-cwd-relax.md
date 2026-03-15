# 2026-03-15: Makefile process management & cwd restriction relaxation

## Changes

### Makefile improvements
- `make run`: stop existing process (via PID file + pgrep) before starting, build binary first
- `make stop`: new target to stop running bot process
- README.md updated to reflect new behavior

### cwd restriction relaxation
- `validateWorkingDir`: allow `/cwd` when `working_dir` is not configured (return nil instead of error)
- Added `TestValidateWorkingDirWithoutBaseDir` and `TestValidateWorkingDirRejectsOutsideBaseDir` tests
