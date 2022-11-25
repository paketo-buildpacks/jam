package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(createStackDescriptor())
}

type createStackDescriptorFlags struct {
	output         string
	nonInteractive bool
}

func createStackDescriptor() *cobra.Command {
	flags := &createStackDescriptorFlags{}
	cmd := &cobra.Command{
		Use:   "create-stack-descriptor",
		Short: "create-stack-descriptor",
		RunE: func(cmd *cobra.Command, args []string) error {
			return createStackDescriptorRun(*flags)
		},
	}
	cmd.Flags().StringVar(&flags.output, "output", "stack.toml", "target path for the generated stack.toml")
	cmd.Flags().BoolVar(&flags.nonInteractive, "non-interactive", false, "do not ask for input")

	return cmd
}

const tomlTemplate = `id = "{{ .ID }}"
homepage = "{{ .Homepage }}"
maintainer = "{{ .Maintainer }}"

platforms = ["linux/amd64"]

[build]
  description = "{{ .RunImageDescription }}"
  dockerfile = "./build.Dockerfile"
  gid = 1000
  shell = "/bin/bash"
  uid = 1001

[run]
  description = "{{ .BuildImageDescription }}"
  dockerfile = "./run.Dockerfile"
  gid = 1000
  shell = "/sbin/nologin"
  uid = 1002
`

type templateInput struct {
	ID                    string
	Homepage              string
	Maintainer            string
	RunImageDescription   string
	BuildImageDescription string
}

func readString(prompt string, fallback string, reader *bufio.Reader) (string, error) {
	fmt.Printf("%s (default %q): ", prompt, fallback)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.Trim(line, "\n")
	if line == "" {
		return fallback, nil
	}
	return line, nil
}

func createStackDescriptorRun(flags createStackDescriptorFlags) error {
	input := templateInput{
		ID:                    "com.example.mystack",
		Homepage:              "https://www.example.com/mystack",
		Maintainer:            "Example Inc.",
		RunImageDescription:   "RunImage for example apps",
		BuildImageDescription: "BuildImage for example apps",
	}
	var err error

	if !flags.nonInteractive {
		reader := bufio.NewReader(os.Stdin)

		input.ID, err = readString("Stack ID", input.ID, reader)
		if err != nil {
			return err
		}

		input.Homepage, err = readString("Homepage", input.Homepage, reader)
		if err != nil {
			return err
		}

		input.Maintainer, err = readString("Maintainer", input.Maintainer, reader)
		if err != nil {
			return err
		}

		input.RunImageDescription, err = readString("Run Image Description", input.RunImageDescription, reader)
		if err != nil {
			return err
		}

		input.BuildImageDescription, err = readString("Build Image Description", input.BuildImageDescription, reader)
		if err != nil {
			return err
		}
	}

	var output io.Writer
	if flags.output == "-" {
		output = os.Stdout
	} else {
		file, err := os.OpenFile(flags.output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer file.Close()
		output = file
	}

	tmpl := template.Must(template.New("stack.toml").Parse(tomlTemplate))
	err = tmpl.Execute(output, input)
	if err != nil {
		return err
	}
	return nil
}
