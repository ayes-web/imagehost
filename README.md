# Imagehost

The program has only one flag, -c, which specifies config file used. All other settings are in the config file.

## Docker

Have a look at example_docker.toml.
By default it uses that config.

## Without docker

Use example_local.toml as base config.

There's a handy build script, it puts assets and binary into bin/build.tar.gz

## S3/B2 bucket

Look in example_s3.toml for what settings to add to your config.
