import pathlib
from typing import List

from pydantic import BaseModel, ConfigDict, Field, field_validator
from pydantic.alias_generators import to_camel

DEFAULT_CONFIG_PATH = (
    pathlib.Path.home() / ".config/linux-home-cloud-storage-sync/config.json"
)
DEFAULT_CLOUD_STORAGE_HOME_DIR = pathlib.Path.home() / "Nextcloud/LinuxHome"
DEFAULT_LOCAL_HOME_DIR = pathlib.Path.home()

class BaseSchema(BaseModel):
    model_config = ConfigDict(
        alias_generator=to_camel,
        serialize_by_alias=True,
        populate_by_name=True,
        from_attributes=True,
        arbitrary_types_allowed=True,
    )


class CloudStorageConfig(BaseSchema):

    cloud_storage_home_dir: pathlib.Path = Field(
        description="Root directory to scan for syncable files/directories",
    )

    local_home_dir: pathlib.Path = Field(
        description="Destionatoin folder. Most times $HOME",
    )

    symlink_map: List[str] = Field(
        description="List of items to generate symlink for."
    )

    @field_validator("cloud_storage_home_dir", mode="before")
    @classmethod
    def _default_cloud_home(cls, value):
        if not value:
            raise ValueError('"cloud_storage_home_dir" was not provided to the model.')
        return value or DEFAULT_CLOUD_STORAGE_HOME_DIR

    @field_validator("local_home_dir", mode="before")
    @classmethod
    def _default_local_home(cls, value):
        if not value:
            raise ValueError('"local_home_dir" was not provided to the model.')
        return value or DEFAULT_LOCAL_HOME_DIR

    @classmethod
    def load(
        cls, path: pathlib.Path | str = DEFAULT_CONFIG_PATH
    ) -> "CloudStorageConfig":
        path = pathlib.Path(path).expanduser()
        if not path.exists():
            raise FileNotFoundError(f"Config file not found: {path}")
        data = path.read_text()
        return cls.model_validate_json(data, strict=True, by_alias=True)

    @classmethod
    def load_defaults(cls) -> "CloudStorageConfig":
        return CloudStorageConfig(
            cloud_storage_home_dir=DEFAULT_CLOUD_STORAGE_HOME_DIR,
            local_home_dir=DEFAULT_LOCAL_HOME_DIR,
            symlink_map=[
                'Desktop',
                'Documents',
                'Music',
                'Pictures',
                'Templates',
                'Videos',
            ]
        )

    def write(self, path: pathlib.Path | str = DEFAULT_CONFIG_PATH) -> None:
        path = pathlib.Path(path).expanduser()
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(self.model_dump_json(indent=4))
