Mynewt Newt Manager Documentation
#################################

This folder holds the documentation for the newtmgr tool for the
`Apache Mynewt`_ project. It is  built using `Sphinx`_.

The complete project documentation can be found at `mynewt documentation`_

.. contents::

Writing Documentation
=======================

`Sphinx`_ use reStructuredText. http://www.sphinx-doc.org/en/1.5.1/rest.html.

Previewing Changes
==========================

In order to preview any changes you make you must first install a Sphinx toolchain as
described at `mynewt documentation`_. Then:

.. code-block:: bash

  $ cd docs
  $ make clean && make preview && (cd _build/html && python -m SimpleHTTPServer 8080)

.. _Apache Mynewt: https://mynewt.apache.org/
.. _mynewt documentation: https://github.com/apache/mynewt-documentation
.. _Sphinx: http://www.sphinx-doc.org/
