PYTHON_BIN ?= python3

.PHONY: presubmit presubmit-ps0 presubmit-ps1 presubmit-ps2

presubmit: presubmit-ps0 presubmit-ps1 presubmit-ps2

presubmit-ps0:
	$(PYTHON_BIN) scripts/ci/presubmit.py PS0

presubmit-ps1:
	$(PYTHON_BIN) scripts/ci/presubmit.py PS1

presubmit-ps2:
	$(PYTHON_BIN) scripts/ci/presubmit.py PS2
