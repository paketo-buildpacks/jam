package internal

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type UpdateBuildpackFlags struct {
	BuildpackFile string
	PackageFile   string
	API           string

	NoCNBRegistry bool
	PatchOnly     bool
}

func UpdateBuildpackRun(flags UpdateBuildpackFlags) error {
	bp, err := ParseBuildpackConfig(flags.BuildpackFile)
	if err != nil {
		return err
	}

	pkg, err := ParsePackageConfig(flags.PackageFile)
	if err != nil {
		return err
	}

	highestFoundSemverBump := "<none>"
	for i, dependency := range pkg.Dependencies {
		var (
			buildpackageID string
			image          Image
			err            error
		)

		if isArchive(dependency) {
			continue
		}

		if flags.NoCNBRegistry {
			// If --patch-only is set, retrieve new version in the same version line as previous version, if it exists
			oldVersion := ""
			if flags.PatchOnly {
				oldVersionSlice := strings.Split(dependency.URI, ":")
				oldVersion = oldVersionSlice[len(oldVersionSlice)-1]
			}

			image, err = FindLatestImage(dependency.URI, oldVersion)
			if err != nil {
				return err
			}
			pkg.Dependencies[i].URI = fmt.Sprintf("%s:%s", image.Name, image.Version)

			buildpackageID, err = GetBuildpackageID(dependency.URI)
			if err != nil {
				return fmt.Errorf("failed to get buildpackage ID for %s: %w", dependency.URI, err)
			}
		} else {
			uri := dependency.URI
			if !isCnbRegistry(dependency) {
				uri, err = GetBuildpackageID(dependency.URI)
				if err != nil {
					return fmt.Errorf("failed to get buildpackage ID for %s: %w", dependency.URI, err)
				}
			}

			// If --patch-only is set, retrieve new version in the same version line as previous version, if it exists
			oldVersion := ""
			if flags.PatchOnly {
				if strings.Contains(dependency.URI, "@") {
					oldVersion = strings.Split(dependency.URI, "@")[1]
				} else {
					oldVersionSlice := strings.Split(dependency.URI, ":")
					oldVersion = oldVersionSlice[len(oldVersionSlice)-1]
				}
			}
			image, err = FindLatestImageOnCNBRegistry(uri, flags.API, oldVersion)
			if err != nil {
				return err
			}

			pkg.Dependencies[i].URI = fmt.Sprintf("%s@%s", image.Name, image.Version)
			buildpackageID = image.Path
		}
		for j, order := range bp.Order {
			for k, group := range order.Group {
				if group.ID == buildpackageID {
					bump, err := semverBump(group.Version, image.Version)
					if err != nil {
						return err
					}
					highestFoundSemverBump = highestSemverBump(highestFoundSemverBump, bump)

					bp.Order[j].Group[k].Version = image.Version
				}
			}
		}
	}

	if err := OverwriteBuildpackConfig(flags.BuildpackFile, bp); err != nil {
		return err
	}

	if err := OverwritePackageConfig(flags.PackageFile, pkg); err != nil {
		return err
	}

	fmt.Printf("Highest semver bump: %s\n", highestFoundSemverBump)

	return nil
}

func semverBump(oldVersion, newVersion string) (string, error) {
	oldSemver, err := semver.StrictNewVersion(oldVersion)
	if err != nil {
		return "", err
	}

	newSemver, err := semver.StrictNewVersion(newVersion)
	if err != nil {
		return "", err
	}

	if newSemver.Major() > oldSemver.Major() {
		return "major", nil
	}

	if newSemver.Minor() > oldSemver.Minor() {
		return "minor", nil
	}

	if newSemver.Patch() > oldSemver.Patch() {
		return "patch", nil
	}

	return "<none>", nil
}

func highestSemverBump(highest, current string) string {
	if highest == "major" {
		return highest
	}

	if highest == "minor" {
		if current == "major" {
			return current
		} else {
			return highest
		}
	}

	if highest == "patch" {
		if current == "major" || current == "minor" {
			return current
		} else {
			return highest
		}
	}

	if highest == "<none>" {
		return current
	}

	return highest
}
