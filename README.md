# `jam`

`jam` is a command-line tool for buildpack authors and users. The `jam` name is simply a play on
the idea of "packaging" or "packing" a buildpack.

`jam` comes with the following commands:
* create-stack        : create a CNB stack
* help                : help about any command
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

### Building stack images on linux

In order to build stack images on linux, you will need to first install
packages to enable emulation of the arm64 instruction set. This is not an issue
on other operating systems (e.g. OSX) because Docker already runs in a Virtual
Machine with emulation support enabled.

For example, to enable emulation on Ubuntu 2204 (Jammy), run the following
command:

```sh
sudo apt-get install qemu binfmt-support qemu-user-static
```
