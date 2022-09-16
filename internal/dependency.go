package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	"github.com/Masterminds/semver/v3"
	"github.com/paketo-buildpacks/packit/v2/cargo"
)

// Dependency represents the structure of a single entry in the dep-server
type Dependency struct {
	DeprecationDate string `json:"deprecation_date,omitempty"`
	// The ID field should be the `name` from the dep-server
	ID             string   `json:"name,omitempty"`
	SHA256         string   `json:"sha256,omitempty"`
	Source         string   `json:"source,omitempty"`
	SourceSHA256   string   `json:"source_sha256,omitempty"`
	Stacks         []Stack  `json:"stacks,omitempty"`
	URI            string   `json:"uri,omitempty"`
	Version        string   `json:"version,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
	ModifedAt      string   `json:"modified_at,omitempty"`
	CPE            string   `json:"cpe,omitempty"`
	PURL           string   `json:"purl,omitempty"`
	Licenses       []string `json:"licenses,omitempty"`
	Checksum       string   `json:"checksum,omitempty"`
	SourceChecksum string   `json:"source-checksum,omitempty"`
}

type Stack struct {
	ID string `json:"id,omitempty"`
}

// GetDependenciesWithinConstraint reaches out to the given API to search for all
// dependencies that match the ID and version constraint of a cargo
// DependencyConstraint. It returns a filtered list of dependencies that match the
// constraint and ID, in order of lowest version to highest.

func GetDependenciesWithinConstraint(dependencies []Dependency, constraint cargo.ConfigMetadataDependencyConstraint, dependencyName string) ([]cargo.ConfigMetadataDependency, error) {
	var matchingDependencies []cargo.ConfigMetadataDependency

	for _, dependency := range dependencies {
		c, err := semver.NewConstraint(constraint.Constraint)
		if err != nil {
			return nil, err
		}

		depVersion, err := semver.NewVersion(dependency.Version)
		if err != nil {
			return nil, err
		}

		if !c.Check(depVersion) || dependency.ID != constraint.ID {
			continue
		}

		matchingDependencies = append(matchingDependencies, convertToCargoDependency(dependency, dependencyName))
	}

	sort.Slice(matchingDependencies, func(i, j int) bool {
		iVersion := semver.MustParse(matchingDependencies[i].Version)
		jVersion := semver.MustParse(matchingDependencies[j].Version)
		return iVersion.LessThan(jVersion)
	})

	// if there are more requested patches than matching dependencies, just
	// return all matching dependencies.
	if constraint.Patches > len(matchingDependencies) {
		return matchingDependencies, nil
	}

	// Buildpack.toml dependencies are usually in order from lowest to highest
	// version. We want to return the the n largest matching dependencies in the
	// same order, n being the constraint.Patches field from the buildpack.toml.
	// Here, we are returning the n highest matching Dependencies.
	return matchingDependencies[len(matchingDependencies)-int(constraint.Patches):], nil
}

// GetCargoDependenciesWithinConstraint receives cargo
// ConfigMetadataDependencies and a cargo DependencyConstraint. It returns a
// filtered list of dependencies that match the constraint and ID, in order of
// lowest version to highest. It will include version duplicates if their
// targets differ, and duplicates do not contribute to the number of patches in the constraints.
func GetCargoDependenciesWithinConstraint(dependencies []cargo.ConfigMetadataDependency, constraint cargo.ConfigMetadataDependencyConstraint) ([]cargo.ConfigMetadataDependency, error) {
	// Use a map to track target variants of the same version
	matchingDependenciesMap := make(map[string][]cargo.ConfigMetadataDependency)
	versions := []string{}

	c, err := semver.NewConstraint(constraint.Constraint)
	if err != nil {
		return nil, err
	}

	for _, dependency := range dependencies {
		depVersion, err := semver.NewVersion(dependency.Version)
		if err != nil {
			return nil, err
		}

		if !c.Check(depVersion) || dependency.ID != constraint.ID {
			continue
		}

		// Migrate from SHA256 and SourceSHA256 to Checksum and SourceChecksum
		if dependency.SHA256 != "" {
			dependency.Checksum = fmt.Sprintf("sha256:%s", dependency.SHA256)
			dependency.SHA256 = ""
		}
		if dependency.SourceSHA256 != "" {
			dependency.SourceChecksum = fmt.Sprintf("sha256:%s", dependency.SourceSHA256)
			dependency.SourceSHA256 = ""
		}

		if matchingDeps, ok := matchingDependenciesMap[dependency.Version]; !ok {
			versions = append(versions, dependency.Version)
			matchingDependenciesMap[dependency.Version] = []cargo.ConfigMetadataDependency{dependency}
		} else {
			// if the map contains the version, but not the specific stack-related variant, add it
			if !containsDependencyWithStack(matchingDeps, dependency.Stacks) {
				matchingDependenciesMap[dependency.Version] = append(matchingDependenciesMap[dependency.Version], dependency)
			}
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		iVersion := semver.MustParse(versions[i])
		jVersion := semver.MustParse(versions[j])
		return iVersion.LessThan(jVersion)
	})

	// Buildpack.toml dependencies are usually in order from lowest to highest
	// version. We want to return the the n largest matching dependencies in the
	// same order, n being the constraint.Patches field from the buildpack.toml.
	// Here, we are returning the n highest matching Dependencies.

	// If there are more requested patches than matching dependencies, just
	// return all matching dependencies.

	i := len(matchingDependenciesMap) - int(constraint.Patches)
	if i < 0 {
		i = 0
	}

	returnSet := []cargo.ConfigMetadataDependency{}
	for ; i < len(matchingDependenciesMap); i++ {
		returnSet = append(returnSet, matchingDependenciesMap[versions[i]]...)
	}

	return returnSet, nil
}

// GetDependencies returns all dependencies from a given API endpoint
func GetAllDependencies(api, dependencyID string) ([]Dependency, error) {
	url := fmt.Sprintf("%s/v1/dependency?name=%s", api, dependencyID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query url %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to query url %s with: status code %d", url, resp.StatusCode)
	}

	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	var dependencies []Dependency
	err = json.Unmarshal(b, &dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return dependencies, nil
}

// FindDependencyName returns the name of a Dependency in a cargo.Config that
// has a matching ID with a given dependency ID.
func FindDependencyName(dependencyID string, config cargo.Config) string {
	name := ""
	for _, dependency := range config.Metadata.Dependencies {
		if dependency.ID == dependencyID {
			name = dependency.Name
			continue
		}
	}
	return name
}

// convertDependency converts an internal.Dependency type into a
// cargo.ConfigMetadataDependency type. It takes in a dependency name as well
// since this isn't a field on the internal.Dependency.
func convertToCargoDependency(dependency Dependency, dependencyName string) cargo.ConfigMetadataDependency {
	var cargoDependency cargo.ConfigMetadataDependency

	if dependency.DeprecationDate != "" {
		deprecationDate, _ := time.Parse(time.RFC3339, dependency.DeprecationDate)
		cargoDependency.DeprecationDate = &deprecationDate
	}

	cargoDependency.CPE = dependency.CPE
	cargoDependency.PURL = dependency.PURL
	cargoDependency.ID = dependency.ID
	cargoDependency.Name = dependencyName
	cargoDependency.Source = dependency.Source
	cargoDependency.URI = dependency.URI
	cargoDependency.Version = strings.Replace(dependency.Version, "v", "", -1)
	cargoDependency.Checksum = dependency.Checksum
	cargoDependency.SourceChecksum = dependency.SourceChecksum

	// Migrate from SHA256 and SourceSHA256 to Checksum and SourceChecksum
	if dependency.SHA256 != "" {
		cargoDependency.Checksum = fmt.Sprintf("sha256:%s", dependency.SHA256)
	}
	if dependency.SourceSHA256 != "" {
		cargoDependency.SourceChecksum = fmt.Sprintf("sha256:%s", dependency.SourceSHA256)
	}

	for _, stack := range dependency.Stacks {
		cargoDependency.Stacks = append(cargoDependency.Stacks, stack.ID)
	}

	for _, l := range dependency.Licenses {
		cargoDependency.Licenses = append(cargoDependency.Licenses, l)
	}

	return cargoDependency
}

// containsVariant determines if a dependency in a given list of cargo
// ConfigMetadataDependency contains the given stack
func containsDependencyWithStack(deps []cargo.ConfigMetadataDependency, stacks []string) bool {
	for _, dep := range deps {
		if slices.Equal(dep.Stacks, stacks) {
			return true
		}
	}
	return false
}
