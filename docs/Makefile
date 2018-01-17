# Make a preview site for Sphinx output

SPHINXOPTS    =
SPHINXBUILD   = sphinx-build
SPHINXPROJ    = Mynewt
SOURCEDIR     = .
BUILDDIR      = _build/sphinx

.PHONY: Makefile clean preview

clean:
	rm -rf _build

preview: _build sphinx

_build:
	mkdir -p _build

sphinx:
	sphinx-build . _build/sphinx
	mv _build/sphinx _build/html/documentation
