# src/linux_home_cloud_storage_sync/__version__.py
"""
A single, importable place for the package version.

Keeping the version in its own module makes it trivial to read it
in your CLI, your tests, or any third‑party tooling that imports
your package.

The string should match the ``version`` field in your *pyproject.toml*
(or ``setup.cfg`` / ``setup.py`` if you’re not using Poetry).

Example:

.. code-block:: python

    >>> from linux_home_cloud_storage_sync import __version__
    >>> print(__version__)          # → "0.1.0"
"""

# ──────────────────────────────────────────────────────────────────────── #
# The version string – **edit this whenever you release a new version**.
# ──────────────────────────────────────────────────────────────────────── #
# __version__: str = "0.1.0"
from importlib.metadata import PackageNotFoundError, version

DISTRIBUTION_NAME = "linux-home-cloud-storage-sync"

try:
    # The distribution's name, not __name__: that's this module
    # ("linux_home_cloud_storage_sync.__version__"), which has no metadata.
    __version__ = version(DISTRIBUTION_NAME)
except PackageNotFoundError:  # pragma: no cover
    # Package is not installed (e.g. running from a source checkout)
    __version__ = "0.0.0+dev"