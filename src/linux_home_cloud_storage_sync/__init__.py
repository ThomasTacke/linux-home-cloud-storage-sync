# src/linux_home_cloud_storage_sync/__init__.py
"""
linux_home_cloud_storage_sync – a tiny CLI for creating symlinks.

The public surface of the package lives in :py:mod:`linux_home_cloud_storage_sync.cloud_storage_config`
and the command‑line entry point in :py:mod:`linux_home_cloud_storage_sync.main`.

This file exposes a small API:

* :data:`__version__` – the package version (taken from ``pyproject.toml``).
* :class:`CloudStorageConfig` – the configuration data‑class.
* :func:`load_defaults` – helper to generate a “template” config.
* :func:`app` – the Typer application defined in :py:mod:`main`.
"""

from importlib.metadata import version, PackageNotFoundError
from .cloud_storage_config import (
    CloudStorageConfig,
    DEFAULT_CLOUD_STORAGE_HOME_DIR,
    DEFAULT_LOCAL_HOME_DIR,
    DEFAULT_CONFIG_PATH
    # … add any other exported names here
)
from .__version__ import __version__
from .main import app  # the Typer application used by `python -m linux_home_cloud_storage_sync`

# --------------------------------------------------------------------------- #
# Public API
# --------------------------------------------------------------------------- #
__all__ = [
    "CloudStorageConfig",
    "DEFAULT_CLOUD_STORAGE_HOME_DIR",
    "DEFAULT_LOCAL_HOME_DIR",
    "DEFAULT_CONFIG_PATH",
    "cli",
    "__version__",
]
