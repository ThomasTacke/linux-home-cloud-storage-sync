import pathlib
from pydantic import ValidationError
from typing_extensions import Annotated
import typer
from rich import print, print_json

from .cloud_storage_config import (
    CloudStorageConfig,
    DEFAULT_CONFIG_PATH,
)
from .__version__ import __version__

app = typer.Typer(
    no_args_is_help=True, context_settings={"help_option_names": ["-h", "--help"]}
)


# ──────────────────────────────────────────────────────────────────────
#  --VERSION OPTION
# ──────────────────────────────────────────────────────────────────────
def _print_version(value: bool) -> None:
    """
    Callback that Typer calls when `--version` is supplied.
    """
    if value:  # pragma: no cover
        # The ``value`` comes from the ``--version`` option below.
        print(f"{__name__} v{__version__}")  # e.g. linux_home_cloud_storage_sync v1.2.3
        raise typer.Exit()  # ← terminate the process


# ``callback`` receives the *boolean* value of the flag.
# ``is_eager=True`` makes Typer stop parsing as soon as the flag is seen,
# so you can call `--version` without having to run any sub‑command.
@app.callback()
def main(
    ctx: typer.Context,
    version: bool = typer.Option(
        None,
        "--version",
        is_eager=True,
        callback=_print_version,
        help="Show program version and exit.",
    ),
):
    # The function body can stay empty – Typer will execute
    # the callback **before** any other command is parsed.
    pass


@app.command(help="Prints the template configuration")
def print_template_config():
    cloud_storage_config = CloudStorageConfig.load_defaults()
    print_json(cloud_storage_config.model_dump_json())


@app.command(help="Writes the template")
def write_default_config(
    path: Annotated[
        pathlib.Path, typer.Argument(help="Overwrite default config path")
    ] = DEFAULT_CONFIG_PATH.as_posix(),
):
    if path.exists():
        if path.is_dir():
            print("Path needs to include filename.")
        else:
            print(f'A configuration file already exists at: "{path}"')
        raise typer.Exit()

    cloud_storage_config = CloudStorageConfig.load_defaults()
    cloud_storage_config.write(path)
    print(f'Default config written to "{path}"')


@app.command(help="Creates symlinks from CloudStorage/LinuxHome to $HOME")
def create_symlinks(
    config_path: Annotated[
        pathlib.Path, typer.Argument(help="Load custom configuration")
    ] = DEFAULT_CONFIG_PATH,
    dry_run: bool = typer.Option(
        False, "--dry-run", "-n", help="Show actions without executing"
    ),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="More output"),
):
    cloud_storage_config: CloudStorageConfig

    if not (config_path.exists() and config_path.is_file()):
        print("No configuration found/given. Continue with default values.")
        cloud_storage_config = CloudStorageConfig.load_defaults()
    else:
        try:
            cloud_storage_config = CloudStorageConfig.load(config_path)
        except ValidationError as e:
            print_json(e.errors())
            raise typer.Exit()

    for item in cloud_storage_config.symlink_map:
        symlink_source: pathlib.Path = (
            cloud_storage_config.cloud_storage_home_dir / item
        )
        symlink_target: pathlib.Path = cloud_storage_config.local_home_dir / item

        if symlink_source.exists() and not symlink_target.exists():
            if dry_run:
                print(
                    f"[yellow]Dry run: would create symlink {symlink_source} -> {symlink_target}[/yellow]"
                )
            else:
                symlink_target.parent.mkdir(parents=True, exist_ok=True)
                symlink_target.symlink_to(symlink_source)
                print(
                    f"[bold]Symlink {symlink_source} -> {symlink_target} created.[/bold]"
                )
        elif not symlink_source.exists():
            print(f'The [bold red]source[/bold red] "{symlink_source}" does not exist.')
        elif symlink_target.exists():
            symlink_target_type: str
            if symlink_target.is_symlink():
                symlink_target_type = "[bold green]symlink[/bold green]"
            elif symlink_target.is_dir():
                symlink_target_type = "[bold red]directory[/bold red]"
            elif symlink_target.is_file():
                symlink_target_type = "[bold red]file[/bold red]"
            else:
                symlink_target_type = "unknown"
            print(
                f'The [bold yellow]target[/bold yellow] "{symlink_target}" already exist and is a {symlink_target_type}'
            )


if __name__ == "__main__":
    app()
