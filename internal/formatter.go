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

type Formatter struct {
	writer io.Writer
}

func NewFormatter(writer io.Writer) Formatter {
	return Formatter{
		writer: writer,
	}
}

type depKey [3]string

func printImplementation(writer io.Writer, config cargo.Config) {
	if len(config.Stacks) > 0 {
		sort.Slice(config.Stacks, func(i, j int) bool {
			return config.Stacks[i].ID < config.Stacks[j].ID
		})

		_, _ = fmt.Fprintf(writer, "### Supported Stacks\n\n")
		for _, s := range config.Stacks {
			_, _ = fmt.Fprintf(writer, "- `%s`\n", s.ID)
		}
		_, _ = fmt.Fprintln(writer)
	}

	if len(config.Metadata.DefaultVersions) > 0 {
		_, _ = fmt.Fprintf(writer, "### Default Dependency Versions\n\n| ID | Version |\n|---|---|\n")
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

			checksum := d.Checksum
			if checksum == "" {
				checksum = fmt.Sprintf("sha256:%s", d.SHA256)
			}

			key := depKey{d.ID, d.Version, checksum}
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

		var sorted []cargo.ConfigMetadataDependency
		for key, stacks := range infoMap {
			sorted = append(sorted, cargo.ConfigMetadataDependency{
				ID:      key[0],
				Version: key[1],
				Stacks:  stacks,
				SHA256:  key[2],
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

		_, _ = fmt.Fprintf(writer, "### Dependencies\n\n| Name | Version | Stacks | Checksum |\n|---|---|---|---|\n")
		for _, d := range sorted {
			_, _ = fmt.Fprintf(writer, "| %s | %s | %s | %s |\n", d.ID, d.Version, strings.Join(d.Stacks, " "), d.SHA256)
		}
		_, _ = fmt.Fprintln(writer)
	}

}

func (f Formatter) Markdown(entries []BuildpackMetadata) {
	//Language-family case
	if len(entries) > 1 {
		var familyMetadata BuildpackMetadata
		for index, entry := range entries {
			if len(entry.Config.Order) > 0 {
				familyMetadata = entry
				entries = append(entries[:index], entries[index+1:]...)
				break
			}
		}

		//Header section
		_, _ = fmt.Fprintf(f.writer, "## %s %s\n\n**ID:** `%s`\n\n", familyMetadata.Config.Buildpack.Name, familyMetadata.Config.Buildpack.Version, familyMetadata.Config.Buildpack.ID)
		_, _ = fmt.Fprintf(f.writer, "**Digest:** `%s`\n\n", familyMetadata.SHA256)
		_, _ = fmt.Fprintf(f.writer, "### Included Buildpackages\n\n")
		_, _ = fmt.Fprintf(f.writer, "| Name | ID | Version |\n|---|---|---|\n")
		for _, entry := range entries {
			_, _ = fmt.Fprintf(f.writer, "| %s | %s | %s |\n", entry.Config.Buildpack.Name, entry.Config.Buildpack.ID, entry.Config.Buildpack.Version)
		}
		//Sub Header
		_, _ = fmt.Fprintf(f.writer, "\n<details>\n<summary>Order Groupings</summary>\n\n")
		for _, o := range familyMetadata.Config.Order {
			_, _ = fmt.Fprintf(f.writer, "| ID | Version | Optional |\n|---|---|---|\n")
			for _, g := range o.Group {
				_, _ = fmt.Fprintf(f.writer, "| %s | %s | %t |\n", g.ID, g.Version, g.Optional)
			}
			_, _ = fmt.Fprintln(f.writer)
		}
		_, _ = fmt.Fprintf(f.writer, "</details>\n\n---\n")

		for _, entry := range entries {
			_, _ = fmt.Fprintf(f.writer, "\n<details>\n<summary>%s %s</summary>\n", entry.Config.Buildpack.Name, entry.Config.Buildpack.Version)
			_, _ = fmt.Fprintf(f.writer, "\n**ID:** `%s`\n\n", entry.Config.Buildpack.ID)
			printImplementation(f.writer, entry.Config)
			_, _ = fmt.Fprintf(f.writer, "---\n\n</details>\n")
		}

	} else { //Implementation case
		_, _ = fmt.Fprintf(f.writer, "## %s %s\n", entries[0].Config.Buildpack.Name, entries[0].Config.Buildpack.Version)
		_, _ = fmt.Fprintf(f.writer, "\n**ID:** `%s`\n\n", entries[0].Config.Buildpack.ID)
		_, _ = fmt.Fprintf(f.writer, "**Digest:** `%s`\n\n", entries[0].SHA256)
		printImplementation(f.writer, entries[0].Config)
	}

}

func (f Formatter) JSON(entries []BuildpackMetadata) {
	var output struct {
		Buildpackage cargo.Config   `json:"buildpackage"`
		Children     []cargo.Config `json:"children,omitempty"`
	}

	output.Buildpackage = entries[0].Config

	if len(entries) > 1 {
		for _, entry := range entries {
			if len(entry.Config.Order) > 0 {
				output.Buildpackage = entry.Config
			} else {
				output.Children = append(output.Children, entry.Config)
			}
		}
	}

	_ = json.NewEncoder(f.writer).Encode(&output)
}
