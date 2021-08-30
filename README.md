# `jam`

`jam` is a command-line tool for buildpack authors and users. The `jam` name is simply a play on
the idea of "packaging" or "packing" a buildpack.

`jam` comes with the following commands:
* help                : Help about any command
* pack                : package buildpack
* summarize           : summarize buildpackage
* update-builder      : update builder
* update-buildpack    : update buildpack
* update-dependencies : update all depdendencies in a buildpack.toml according to metadata.constraints

The `jam` executable can be installed by downloading the latest version from
the [Releases](../../releases) page. Once downloaded, buildpacks can be created from
a source repository using the `pack` command like this:

```sh
jam pack \
  --buildpack ./buildpack.toml \
  --stack io.paketo.stacks.tiny \
  --version 1.2.3 \
  --offline \
  --output ./buildpack.tgz
```
