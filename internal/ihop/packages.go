package ihop

import "fmt"

// Packages represents the set of packages contained in the build and run
// images.
type Packages struct {
	// Intersection is the set of packages that appear in both the build and run
	// image.
	Intersection []string

	// BuildComplement is the set of package that appear only in the build image.
	BuildComplement []string

	// RunComplement is the set of package that appear only in the run image.
	RunComplement []string
}

// NewPackages returns a Packages that includes the intersection and
// complements for each of the build and run image packages lists.
func NewPackages(build, run []string) Packages {
	// convert the build slice into a map for easy lookup
	buildValues := make(map[string]struct{})
	for _, value := range build {
		buildValues[value] = struct{}{}
	}

	// convert the run slice into a map for easy lookup
	runValues := make(map[string]struct{})
	for _, value := range run {
		runValues[value] = struct{}{}
	}

	// iterating over all packages in the build image
	var p Packages
	for value := range buildValues {
		//if the package also appears in the run image, append it to the
		//intersection slice, otherwise prefix it with 'build:' and append it to
		//the build complement slice
		if _, ok := runValues[value]; ok {
			p.Intersection = append(p.Intersection, value)
			delete(buildValues, value)
			delete(runValues, value)
		} else {
			p.BuildComplement = append(p.BuildComplement, fmt.Sprintf("build:%s", value))
		}
	}

	// any packages remaining in the runValues map at this point must only appear
	// in the run image and so should be added to the run complement slice
	for value := range runValues {
		p.RunComplement = append(p.RunComplement, fmt.Sprintf("run:%s", value))
	}

	return p
}
