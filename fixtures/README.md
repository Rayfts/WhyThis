# Synthetic archaeology fixture

`build.sh` creates a disposable Git repository whose history intentionally contains a feature, regression fix, test addition, revert, rename, refactor and reimplementation. Tests create their own smaller fixture so CI does not depend on shell-specific fixture state.

The fixture exists to prove evidence collection against known history. It is not shipped as product data.
