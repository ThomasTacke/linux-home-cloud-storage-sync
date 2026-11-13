import json
import pathlib

import pytest
from typer.testing import CliRunner

from linux_home_cloud_storage_sync.__version__ import __version__
from linux_home_cloud_storage_sync.main import app

runner = CliRunner()
# Wide enough that Rich doesn't wrap the long tmp paths in the output
ENV = {"COLUMNS": "1000"}


@pytest.fixture
def dirs(tmp_path: pathlib.Path) -> tuple[pathlib.Path, pathlib.Path]:
    cloud = tmp_path / "cloud"
    home = tmp_path / "home"
    cloud.mkdir()
    home.mkdir()
    return cloud, home


def write_config(path: pathlib.Path, cloud, home, items: list[str]) -> pathlib.Path:
    path.write_text(
        json.dumps(
            {
                "cloudStorageHomeDir": str(cloud),
                "localHomeDir": str(home),
                "symlinkMap": items,
            }
        )
    )
    return path


def run(*args: str):
    return runner.invoke(app, list(args), env=ENV)


def test_creates_missing_link(tmp_path, dirs):
    cloud, home = dirs
    (cloud / "Documents").mkdir()
    config = write_config(tmp_path / "config.json", cloud, home, ["Documents"])

    result = run("create-symlinks", str(config))

    assert result.exit_code == 0, result.output
    assert (home / "Documents").is_symlink()
    assert (home / "Documents").readlink() == cloud / "Documents"


def test_creates_parent_folders(tmp_path, dirs):
    cloud, home = dirs
    (cloud / ".config" / "fish").mkdir(parents=True)
    config = write_config(tmp_path / "config.json", cloud, home, [".config/fish"])

    result = run("create-symlinks", str(config))

    assert result.exit_code == 0, result.output
    assert (home / ".config" / "fish").is_symlink()


def test_dry_run_creates_nothing(tmp_path, dirs):
    cloud, home = dirs
    (cloud / "Documents").mkdir()
    config = write_config(tmp_path / "config.json", cloud, home, ["Documents"])

    result = run("create-symlinks", "--dry-run", str(config))

    assert result.exit_code == 0, result.output
    assert "would create symlink" in result.output
    assert not (home / "Documents").exists()
    assert not (home / "Documents").is_symlink()


def test_correct_link_is_reported_as_symlink(tmp_path, dirs):
    cloud, home = dirs
    (cloud / "Documents").mkdir()
    (home / "Documents").symlink_to(cloud / "Documents")
    config = write_config(tmp_path / "config.json", cloud, home, ["Documents"])

    result = run("create-symlinks", str(config))

    assert result.exit_code == 0, result.output
    assert "already exist and is a symlink" in result.output


def test_broken_link_is_reported_not_crashing(tmp_path, dirs):
    """#1: a dangling link used to look "missing" and crash symlink_to() with FileExistsError."""
    cloud, home = dirs
    (cloud / "Documents").mkdir()
    (cloud / "Music").mkdir()
    (home / "Documents").symlink_to(tmp_path / "gone")
    config = write_config(
        tmp_path / "config.json", cloud, home, ["Documents", "Music"]
    )

    result = run("create-symlinks", str(config))

    assert result.exit_code == 0, result.output
    assert "broken symlink" in result.output
    # left alone ...
    assert (home / "Documents").readlink() == tmp_path / "gone"
    # ... and the entries after it are still processed
    assert (home / "Music").is_symlink()


def test_link_to_elsewhere_is_reported(tmp_path, dirs):
    """#2: a link pointing somewhere else used to be reported as a fine symlink."""
    cloud, home = dirs
    (cloud / "Documents").mkdir()
    elsewhere = tmp_path / "elsewhere"
    elsewhere.mkdir()
    (home / "Documents").symlink_to(elsewhere)
    config = write_config(tmp_path / "config.json", cloud, home, ["Documents"])

    result = run("create-symlinks", str(config))

    assert result.exit_code == 0, result.output
    assert f'symlink to "{elsewhere}", not "{cloud / "Documents"}"' in result.output
    assert (home / "Documents").readlink() == elsewhere


def test_relative_link_to_source_counts_as_correct(tmp_path, dirs):
    cloud, home = dirs
    (cloud / "Documents").mkdir()
    (home / "Documents").symlink_to(pathlib.Path("..") / "cloud" / "Documents")
    config = write_config(tmp_path / "config.json", cloud, home, ["Documents"])

    result = run("create-symlinks", str(config))

    assert result.exit_code == 0, result.output
    assert "already exist and is a symlink" in result.output


def test_existing_directory_is_left_alone(tmp_path, dirs):
    cloud, home = dirs
    (cloud / "Documents").mkdir()
    (home / "Documents").mkdir()
    config = write_config(tmp_path / "config.json", cloud, home, ["Documents"])

    result = run("create-symlinks", str(config))

    assert result.exit_code == 0, result.output
    assert "already exist and is a directory" in result.output
    assert not (home / "Documents").is_symlink()


def test_missing_source_is_reported(tmp_path, dirs):
    cloud, home = dirs
    config = write_config(tmp_path / "config.json", cloud, home, ["Documents"])

    result = run("create-symlinks", str(config))

    assert result.exit_code == 0, result.output
    assert "does not exist" in result.output
    assert not (home / "Documents").is_symlink()


def test_invalid_config_prints_errors_and_fails(tmp_path):
    """#3: printing the validation errors used to crash with a TypeError."""
    config = tmp_path / "config.json"
    config.write_text("{}")

    result = run("create-symlinks", str(config))

    assert result.exit_code == 1, result.output
    assert isinstance(result.exception, SystemExit)
    assert "Invalid configuration" in result.output
    assert "symlinkMap: Field required" in result.output


def test_tilde_in_config_means_home(tmp_path, monkeypatch):
    """#5: "~/..." in the config used to be taken literally."""
    monkeypatch.setenv("HOME", str(tmp_path))
    (tmp_path / "Nextcloud" / "LinuxHome" / "Documents").mkdir(parents=True)
    config = write_config(
        tmp_path / "config.json", "~/Nextcloud/LinuxHome", "~", ["Documents"]
    )

    result = run("create-symlinks", str(config))

    assert result.exit_code == 0, result.output
    assert (tmp_path / "Documents").readlink() == (
        tmp_path / "Nextcloud" / "LinuxHome" / "Documents"
    )


def test_version():
    """#4: --version used to print 0.0.0+dev and the module name."""
    result = run("--version")

    assert result.exit_code == 0, result.output
    assert __version__ != "0.0.0+dev"
    assert result.output.strip() == f"linux-home-cloud-storage-sync v{__version__}"
