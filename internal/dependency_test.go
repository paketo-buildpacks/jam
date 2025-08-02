package internal_test

import (
	"testing"

	"github.com/paketo-buildpacks/jam/v2/internal"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDependency(t *testing.T, context spec.G, it spec.S) {
	var (
		withT           = NewWithT(t)
		Expect          = withT.Expect
		allDependencies []internal.Dependency
	)

	it.Before(func() {
		allDependencies = []internal.Dependency{
			{
				DeprecationDate: "",
				ID:              "some-dep",
				SHA256:          "some-sha",
				Source:          "some-source",
				SourceSHA256:    "some-source-sha",
				Stacks: []internal.Stack{
					{
						ID: "some-stack",
					},
				},
				URI:       "some-dep-uri",
				Version:   "v1.0.0",
				CreatedAt: "sometime",
				ModifedAt: "another-time",
				CPE:       "cpe-notation",
				PURL:      "some-purl",
				Licenses: []string{
					"fancy-license",
					"fancy-license-2",
				},
			},
			{
				DeprecationDate: "",
				ID:              "some-dep",
				SHA256:          "some-sha-two",
				Source:          "some-source-two",
				SourceSHA256:    "some-source-sha-two",
				Stacks: []internal.Stack{
					{
						ID: "some-stack-two",
					},
				},
				URI:       "some-dep-uri-two",
				Version:   "v1.1.2",
				CreatedAt: "sometime",
				ModifedAt: "another-time",
				CPE:       "cpe-notation",
				PURL:      "some-purl",
				Licenses: []string{
					"fancy-license",
					"fancy-license-2",
				},
			},
			{
				DeprecationDate: "",
				ID:              "some-dep",
				SHA256:          "some-sha-three",
				Source:          "some-source-three",
				SourceSHA256:    "some-source-sha-three",
				Stacks: []internal.Stack{
					{
						ID: "some-stack-three",
					},
				},
				URI:       "some-dep-uri-three",
				Version:   "v1.5.6",
				CreatedAt: "sometime",
				ModifedAt: "another-time",
				CPE:       "cpe-notation",
				PURL:      "some-purl",
				Licenses: []string{
					"fancy-license",
					"fancy-license-2",
				},
			},
			{
				DeprecationDate: "",
				ID:              "some-dep",
				SHA256:          "some-sha-four",
				Source:          "some-source-four",
				SourceSHA256:    "some-source-sha-four",
				Stacks: []internal.Stack{
					{
						ID: "some-stack-four",
					},
				},
				URI:       "some-dep-uri-four",
				Version:   "v2.3.2",
				CreatedAt: "sometime",
				ModifedAt: "another-time",
				CPE:       "cpe-notation",
				PURL:      "some-purl",
				Licenses: []string{
					"fancy-license",
					"fancy-license-2",
				},
			},
			{
				DeprecationDate: "",
				ID:              "different-dep",
				SHA256:          "different-dep-sha",
				Source:          "different-dep-source",
				SourceSHA256:    "different-dep-source-sha",
				Stacks: []internal.Stack{
					{
						ID: "different-dep-stack",
					},
				},
				URI:       "different-dep-uri",
				Version:   "v1.9.8",
				CreatedAt: "sometime",
				ModifedAt: "another-time",
				CPE:       "cpe-notation",
				PURL:      "some-purl",
				Licenses: []string{
					"fancy-license",
					"fancy-license-2",
				},
			},
			{
				DeprecationDate: "",
				ID:              "some-dep",
				Checksum:        "sha512:some-sha",
				Source:          "some-512-source",
				SourceChecksum:  "sha512:some-source-sha",
				Stacks: []internal.Stack{
					{
						ID: "some-stack",
					},
				},
				URI:       "some-512-dep-uri",
				Version:   "v1.6.7",
				CreatedAt: "sometime",
				ModifedAt: "another-time",
				CPE:       "cpe-notation",
				PURL:      "some-512-purl",
				Licenses: []string{
					"fancy-license",
					"fancy-license-2",
				},
			},
		}
	})

	context("GetDependenciesWithinConstraint", func() {
		context("given a valid api and constraint", func() {
			it("returns a sorted list of dependencies that match the constraint", func() {
				constraint := cargo.ConfigMetadataDependencyConstraint{
					Constraint: "1.*",
					ID:         "some-dep",
					Patches:    4,
				}

				dependencies, err := internal.GetDependenciesWithinConstraint(allDependencies, constraint, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(dependencies).To(Equal([]cargo.ConfigMetadataDependency{
					{
						CPE:          "cpe-notation",
						PURL:         "some-purl",
						ID:           "some-dep",
						Licenses:     []interface{}{"fancy-license", "fancy-license-2"},
						Version:      "1.0.0",
						Stacks:       []string{"some-stack"},
						URI:          "some-dep-uri",
						SHA256:       "some-sha",
						Source:       "some-source",
						SourceSHA256: "some-source-sha",
					},
					{
						CPE:          "cpe-notation",
						PURL:         "some-purl",
						ID:           "some-dep",
						Licenses:     []interface{}{"fancy-license", "fancy-license-2"},
						Version:      "1.1.2",
						Stacks:       []string{"some-stack-two"},
						URI:          "some-dep-uri-two",
						SHA256:       "some-sha-two",
						Source:       "some-source-two",
						SourceSHA256: "some-source-sha-two",
					},
					{
						CPE:          "cpe-notation",
						PURL:         "some-purl",
						ID:           "some-dep",
						Licenses:     []interface{}{"fancy-license", "fancy-license-2"},
						Version:      "1.5.6",
						Stacks:       []string{"some-stack-three"},
						URI:          "some-dep-uri-three",
						SHA256:       "some-sha-three",
						Source:       "some-source-three",
						SourceSHA256: "some-source-sha-three",
					},
					{
						CPE:            "cpe-notation",
						PURL:           "some-512-purl",
						ID:             "some-dep",
						Licenses:       []interface{}{"fancy-license", "fancy-license-2"},
						Version:        "1.6.7",
						Stacks:         []string{"some-stack"},
						URI:            "some-512-dep-uri",
						Checksum:       "sha512:some-sha",
						Source:         "some-512-source",
						SourceChecksum: "sha512:some-source-sha",
					},
				}))
			})
		})

		context("failure cases", func() {
			context("given an invalid constraint", func() {
				it("returns an error", func() {
					constraint := cargo.ConfigMetadataDependencyConstraint{
						Constraint: "abc",
						ID:         "some-dep",
						Patches:    3,
					}

					_, err := internal.GetDependenciesWithinConstraint(allDependencies, constraint, "")
					Expect(err).To(MatchError("improper constraint: abc"))
				})
			})

			context("given a malformed dependency version", func() {
				it("returns an error", func() {
					constraint := cargo.ConfigMetadataDependencyConstraint{
						Constraint: "1.*",
						ID:         "some-dep",
						Patches:    3,
					}
					dependencies := []internal.Dependency{
						{
							DeprecationDate: "",
							ID:              "some-dep",
							SHA256:          "some-sha",
							Source:          "some-source",
							SourceSHA256:    "some-source-sha",
							Stacks: []internal.Stack{
								{
									ID: "some-stack",
								},
							},
							URI:       "some-dep-uri",
							Version:   "v1.xx",
							CreatedAt: "sometime",
							ModifedAt: "another-time",
							CPE:       "cpe-notation",
							PURL:      "some-purl",
							Licenses:  []string{"fancy-license", "fancy-license-2"},
						},
					}

					_, err := internal.GetDependenciesWithinConstraint(dependencies, constraint, "")
					Expect(err).To(MatchError("invalid semantic version"))
				})
			})
		})
	})

	context("GetCargoDependenciesWithinConstraint", func() {
		var allCargoDependencies []cargo.ConfigMetadataDependency

		it.Before(func() {
			allCargoDependencies = []cargo.ConfigMetadataDependency{
				{
					ID:           "some-dep",
					SHA256:       "some-sha",
					Source:       "some-source",
					SourceSHA256: "some-source-sha",
					Stacks: []string{
						"some-stack",
					},
					URI:      "some-dep-uri",
					Version:  "1.0.0",
					CPE:      "cpe-notation",
					PURL:     "some-purl",
					Licenses: []interface{}{"fancy-license", "fancy-license-2"},
				},
				{
					ID:           "some-dep",
					SHA256:       "some-sha-two",
					Source:       "some-source-two",
					SourceSHA256: "some-source-sha-two",
					Stacks: []string{
						"some-stack",
						"some-stack-two",
					},
					URI:      "some-dep-uri-two-noarch",
					Version:  "1.1.2",
					CPE:      "cpe-notation",
					PURL:     "some-purl",
					Licenses: []interface{}{"fancy-license", "fancy-license-2"},
				},
				{
					ID:           "some-dep",
					SHA256:       "some-sha-two",
					Source:       "some-source-two",
					SourceSHA256: "some-source-sha-two",
					Stacks: []string{
						"some-stack-two",
					},
					URI:      "some-dep-uri-two",
					Version:  "1.1.2",
					CPE:      "cpe-notation",
					PURL:     "some-purl",
					Licenses: []interface{}{"fancy-license", "fancy-license-2"},
					OS:       "some-os",
					Arch:     "some-arch",
				},
				{
					ID:           "some-dep",
					SHA256:       "some-sha-two",
					Source:       "some-source-two",
					SourceSHA256: "some-source-sha-two",
					Stacks: []string{
						"some-stack-two",
					},
					URI:      "some-dep-uri-two",
					Version:  "1.1.2",
					CPE:      "cpe-notation",
					PURL:     "some-purl",
					Licenses: []interface{}{"fancy-license", "fancy-license-2"},
					OS:       "some-os",
					Arch:     "some-other-arch",
				},
				{
					ID:           "some-dep",
					SHA256:       "some-sha-three",
					Source:       "some-source-three",
					SourceSHA256: "some-source-sha-three",
					Stacks: []string{
						"some-stack-three",
					},
					URI:      "some-dep-uri-three",
					Version:  "1.5.6",
					CPE:      "cpe-notation",
					PURL:     "some-purl",
					Licenses: []interface{}{"fancy-license", "fancy-license-2"},
					OS:       "some-os",
				},
				{
					ID:           "some-dep",
					SHA256:       "some-sha-four",
					Source:       "some-source-four",
					SourceSHA256: "some-source-sha-four",
					Stacks: []string{
						"some-stack-four",
					},
					URI:      "some-dep-uri-four",
					Version:  "2.3.2",
					CPE:      "cpe-notation",
					PURL:     "some-purl",
					Licenses: []interface{}{"fancy-license", "fancy-license-2"},
				},
				{
					ID:           "different-dep",
					SHA256:       "different-dep-sha",
					Source:       "different-dep-source",
					SourceSHA256: "different-dep-source-sha",
					Stacks: []string{
						"different-dep-stack",
					},
					URI:      "different-dep-uri",
					Version:  "1.9.8",
					CPE:      "cpe-notation",
					PURL:     "some-purl",
					Licenses: []interface{}{"fancy-license", "fancy-license-2"},
				},
				{
					ID:             "some-dep",
					Checksum:       "sha512:some-sha",
					Source:         "some-source",
					SourceChecksum: "sha512:some-source-sha",
					Stacks: []string{
						"some-stack",
					},
					URI:      "some-dep-uri",
					Version:  "1.6.7",
					CPE:      "cpe-notation",
					PURL:     "some-purl",
					Licenses: []interface{}{"fancy-license", "fancy-license-2"},
					OS:       "some-os",
					Arch:     "some-arch",
				},
				{
					ID:             "some-dep",
					Checksum:       "sha512:some-sha",
					Source:         "some-source",
					SourceChecksum: "sha512:some-source-sha",
					Stacks: []string{
						"some-stack",
					},
					URI:      "some-dep-uri",
					Version:  "1.6.7",
					CPE:      "cpe-notation",
					PURL:     "some-purl",
					Licenses: []interface{}{"fancy-license", "fancy-license-2"},
					OS:       "some-os",
					Arch:     "some-other-arch",
				},
			}
		})

		context("given a list of cargo dependencies and constraint", func() {
			it("returns a sorted list of dependencies that match the constraint, including version duplicates that differ by stack", func() {
				constraint := cargo.ConfigMetadataDependencyConstraint{
					Constraint: "1.*",
					ID:         "some-dep",
					Patches:    4,
				}

				dependencies, err := internal.GetCargoDependenciesWithinConstraint(allCargoDependencies, constraint)
				Expect(err).NotTo(HaveOccurred())
				Expect(dependencies).To(Equal([]cargo.ConfigMetadataDependency{
					{
						CPE:          "cpe-notation",
						PURL:         "some-purl",
						ID:           "some-dep",
						Licenses:     []interface{}{"fancy-license", "fancy-license-2"},
						Version:      "1.0.0",
						Stacks:       []string{"some-stack"},
						URI:          "some-dep-uri",
						SHA256:       "some-sha",
						Source:       "some-source",
						SourceSHA256: "some-source-sha",
					},
					{
						CPE:          "cpe-notation",
						PURL:         "some-purl",
						ID:           "some-dep",
						Licenses:     []interface{}{"fancy-license", "fancy-license-2"},
						Version:      "1.1.2",
						Stacks:       []string{"some-stack", "some-stack-two"},
						URI:          "some-dep-uri-two-noarch",
						SHA256:       "some-sha-two",
						Source:       "some-source-two",
						SourceSHA256: "some-source-sha-two",
					},
					{
						CPE:          "cpe-notation",
						PURL:         "some-purl",
						ID:           "some-dep",
						Licenses:     []interface{}{"fancy-license", "fancy-license-2"},
						Version:      "1.1.2",
						Stacks:       []string{"some-stack-two"},
						URI:          "some-dep-uri-two",
						SHA256:       "some-sha-two",
						Source:       "some-source-two",
						SourceSHA256: "some-source-sha-two",
						OS:           "some-os",
						Arch:         "some-arch",
					},
					{
						CPE:          "cpe-notation",
						PURL:         "some-purl",
						ID:           "some-dep",
						Licenses:     []interface{}{"fancy-license", "fancy-license-2"},
						Version:      "1.1.2",
						Stacks:       []string{"some-stack-two"},
						URI:          "some-dep-uri-two",
						SHA256:       "some-sha-two",
						Source:       "some-source-two",
						SourceSHA256: "some-source-sha-two",
						OS:           "some-os",
						Arch:         "some-other-arch",
					},
					{
						CPE:          "cpe-notation",
						PURL:         "some-purl",
						ID:           "some-dep",
						Licenses:     []interface{}{"fancy-license", "fancy-license-2"},
						Version:      "1.5.6",
						Stacks:       []string{"some-stack-three"},
						URI:          "some-dep-uri-three",
						SHA256:       "some-sha-three",
						Source:       "some-source-three",
						SourceSHA256: "some-source-sha-three",
						OS:           "some-os",
					},
					{
						CPE:            "cpe-notation",
						PURL:           "some-purl",
						ID:             "some-dep",
						Licenses:       []interface{}{"fancy-license", "fancy-license-2"},
						Version:        "1.6.7",
						Stacks:         []string{"some-stack"},
						URI:            "some-dep-uri",
						Checksum:       "sha512:some-sha",
						Source:         "some-source",
						SourceChecksum: "sha512:some-source-sha",
						OS:             "some-os",
						Arch:           "some-arch",
					},
					{
						CPE:            "cpe-notation",
						PURL:           "some-purl",
						ID:             "some-dep",
						Licenses:       []interface{}{"fancy-license", "fancy-license-2"},
						Version:        "1.6.7",
						Stacks:         []string{"some-stack"},
						URI:            "some-dep-uri",
						Checksum:       "sha512:some-sha",
						Source:         "some-source",
						SourceChecksum: "sha512:some-source-sha",
						OS:             "some-os",
						Arch:           "some-other-arch",
					},
				}))
			})
		})

		context("given a list of cargo dependencies with invalid stack variant duplicates and constraint", func() {
			it.Before(func() {
				allCargoDependencies = []cargo.ConfigMetadataDependency{
					{
						ID:           "some-dep",
						SHA256:       "some-sha",
						Source:       "some-source",
						SourceSHA256: "some-source-sha",
						Stacks: []string{
							"some-stack",
						},
						URI:      "some-dep-uri",
						Version:  "1.0.0",
						CPE:      "cpe-notation",
						PURL:     "some-purl",
						Licenses: []interface{}{"fancy-license", "fancy-license-2"},
					},
					{
						ID:           "some-dep",
						SHA256:       "some-sha",
						Source:       "some-source",
						SourceSHA256: "some-source-sha",
						Stacks: []string{
							"some-stack",
						},
						URI:      "some-dep-uri",
						Version:  "1.0.0",
						CPE:      "cpe-notation",
						PURL:     "some-purl",
						Licenses: []interface{}{"fancy-license", "fancy-license-2"},
					},
					{
						ID:           "some-dep",
						SHA256:       "some-sha",
						Source:       "some-source",
						SourceSHA256: "some-source-sha",
						Stacks: []string{
							"different-stack",
						},
						URI:      "some-dep-uri",
						Version:  "1.0.0",
						CPE:      "cpe-notation",
						PURL:     "some-purl",
						Licenses: []interface{}{"fancy-license", "fancy-license-2"},
					},
				}
			})
			it("returns a sorted list of dependencies that match the constraint, excluding invalid stack variant duplicates", func() {
				constraint := cargo.ConfigMetadataDependencyConstraint{
					Constraint: "1.*",
					ID:         "some-dep",
					Patches:    1,
				}

				dependencies, err := internal.GetCargoDependenciesWithinConstraint(allCargoDependencies, constraint)
				Expect(err).NotTo(HaveOccurred())
				Expect(dependencies).To(Equal([]cargo.ConfigMetadataDependency{
					{
						CPE:          "cpe-notation",
						PURL:         "some-purl",
						ID:           "some-dep",
						Licenses:     []interface{}{"fancy-license", "fancy-license-2"},
						Version:      "1.0.0",
						Stacks:       []string{"some-stack"},
						URI:          "some-dep-uri",
						SHA256:       "some-sha",
						Source:       "some-source",
						SourceSHA256: "some-source-sha",
					},
					{
						CPE:          "cpe-notation",
						PURL:         "some-purl",
						ID:           "some-dep",
						Licenses:     []interface{}{"fancy-license", "fancy-license-2"},
						Version:      "1.0.0",
						Stacks:       []string{"different-stack"},
						URI:          "some-dep-uri",
						SHA256:       "some-sha",
						Source:       "some-source",
						SourceSHA256: "some-source-sha",
					},
				}))
			})
		})

		context("failure cases", func() {
			context("given an invalid constraint", func() {
				it("returns an error", func() {
					constraint := cargo.ConfigMetadataDependencyConstraint{
						Constraint: "abc",
						ID:         "some-dep",
						Patches:    3,
					}

					_, err := internal.GetCargoDependenciesWithinConstraint(allCargoDependencies, constraint)
					Expect(err).To(MatchError("improper constraint: abc"))
				})
			})

			context("given a malformed dependency version", func() {
				it("returns an error", func() {
					constraint := cargo.ConfigMetadataDependencyConstraint{
						Constraint: "1.*",
						ID:         "some-dep",
						Patches:    3,
					}
					cargoDependencies := []cargo.ConfigMetadataDependency{
						{
							ID:           "some-dep",
							SHA256:       "some-sha",
							Source:       "some-source",
							SourceSHA256: "some-source-sha",
							Stacks: []string{
								"some-stack",
							},
							URI:      "some-dep-uri",
							Version:  "v1.xx",
							CPE:      "cpe-notation",
							PURL:     "some-purl",
							Licenses: []interface{}{"fancy-license", "fancy-license-2"},
						},
					}
					_, err := internal.GetCargoDependenciesWithinConstraint(cargoDependencies, constraint)
					Expect(err).To(MatchError("invalid semantic version"))
				})
			})
		})
	})

	context("FindDependencyName", func() {
		var cargoConfig cargo.Config
		it.Before(func() {
			cargoConfig = cargo.Config{
				API: "0.2",
				Buildpack: cargo.ConfigBuildpack{
					ID:       "some-buildpack-id",
					Name:     "some-buildpack-name",
					Version:  "some-buildpack-version",
					Homepage: "some-homepage-link",
				},
				Metadata: cargo.ConfigMetadata{
					Dependencies: []cargo.ConfigMetadataDependency{
						{
							ID:      "some-dependency",
							Name:    "Some Dependency Name",
							URI:     "http://some-url",
							Version: "1.2.3",
						},
					},
				},
			}
		})

		context("given a dependency ID and valid cargo.Config that contain that dependency", func() {
			it("returns the name of the dependency from the Config", func() {
				name := internal.FindDependencyName("some-dependency", cargoConfig)
				Expect(name).To(Equal("Some Dependency Name"))
			})
		})

		context("given a dependency ID and a cargo.Config that does not contain that dependency", func() {
			it("returns the empty string", func() {
				name := internal.FindDependencyName("unmatched-dependency", cargoConfig)
				Expect(name).To(Equal(""))
			})
		})
	})
}
