package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/paketo-buildpacks/packit/v2/cargo"
)

type ExtensionFormatter struct {
	writer io.Writer
}

func NewExtensionFormatter(writer io.Writer) ExtensionFormatter {
	return ExtensionFormatter{
		writer: writer,
	}
}

func printExtensionImplementation(writer io.Writer, config cargo.ExtensionConfig) {

	if len(config.Metadata.DefaultVersions) > 0 {
		_, _ = fmt.Fprintf(writer, "#### Default Dependency Versions:\n| ID | Version |\n|---|---|\n")
		var sortedDependencies []string
		for key := range config.Metadata.DefaultVersions {
			sortedDependencies = append(sortedDependencies, key)
		}

		sort.Strings(sortedDependencies)

		for _, key := range sortedDependencies {
			_, _ = fmt.Fprintf(writer, "| %s | %s |\n", key, config.Metadata.DefaultVersions[key])
		}
		_, _ = fmt.Fprintln(writer)
	}

	if len(config.Metadata.Dependencies) > 0 {
		infoMap := map[depKey][]string{}
		for _, d := range config.Metadata.Dependencies {

			key := depKey{d.ID, d.Version, d.Source}
			_, ok := infoMap[key]
			if !ok {
				sort.Strings(d.Stacks)
				infoMap[key] = d.Stacks
			} else {
				val := infoMap[key]
				val = append(val, d.Stacks...)
				sort.Strings(val)
				infoMap[key] = val
			}
		}

		var sorted []cargo.ConfigExtensionMetadataDependency
		for key, stacks := range infoMap {
			sorted = append(sorted, cargo.ConfigExtensionMetadataDependency{
				ID:      key[0],
				Version: key[1],
				Stacks:  stacks,
				Source:  key[2],
			})
		}

		sort.Slice(sorted, func(i, j int) bool {
			iVal := sorted[i]
			jVal := sorted[j]

			if iVal.ID == jVal.ID {
				iVersion := semver.MustParse(iVal.Version)
				jVersion := semver.MustParse(jVal.Version)

				if iVersion.Equal(jVersion) {
					iStacks := strings.Join(iVal.Stacks, " ")
					jStacks := strings.Join(jVal.Stacks, " ")

					return iStacks < jStacks
				}

				return iVersion.GreaterThan(jVersion)
			}

			return iVal.ID < jVal.ID
		})

		_, _ = fmt.Fprintf(writer, "#### Dependencies:\n| Name | Version | Stacks | Source |\n|---|---|---|---|\n")
		for _, d := range sorted {
			_, _ = fmt.Fprintf(writer, "| %s | %s | %s | %s |\n", d.ID, d.Version, strings.Join(d.Stacks, " "), d.Source)
		}
		_, _ = fmt.Fprintln(writer)
	}

}

func (f ExtensionFormatter) Markdown(entries []ExtensionMetadata) {

	_, _ = fmt.Fprintf(f.writer, "## %s %s\n", entries[0].Config.Extension.Name, entries[0].Config.Extension.Version)
	_, _ = fmt.Fprintf(f.writer, "\n**ID:** `%s`\n\n", entries[0].Config.Extension.ID)
	_, _ = fmt.Fprintf(f.writer, "**Digest:** `%s`\n\n", entries[0].SHA256)
	printExtensionImplementation(f.writer, entries[0].Config)

}

func (f ExtensionFormatter) JSON(entries []ExtensionMetadata) {
	var output struct {
		Buildpackage cargo.ExtensionConfig `json:"buildpackage"`
	}

	output.Buildpackage = entries[0].Config

	_ = json.NewEncoder(f.writer).Encode(&output)
}
