package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type args struct {
	release string
	namespace string
	valuesFiles []string
	templateFile string
	outputLocation string
}

type runner struct {
	args args
	values map[string]interface{}
}

// TODO this assumes that all values look like straight {{ blah }} and not {{ bla | blah }} or {{ include blah }} etc.
func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run() error {
	r := &runner{args: args{outputLocation: "./calico-manifest.yaml"}}
	var cmd = &cobra.Command{
		Use:          "template-calico",
		Short:        "Fills and outputs a calico manifest template",
		RunE: func(cmd *cobra.Command, args []string) error {
			return r.start()
		},
	}

	cmd.Flags().StringVar(&r.args.release, "release", r.args.release, "Helm release name")
	cmd.MarkFlagRequired("release")
	cmd.Flags().StringVarP(&r.args.namespace, "namespace", "n", r.args.namespace, "Helm release namespace")
	cmd.MarkFlagRequired("namespace")
	cmd.PersistentFlags().StringArrayVarP(&r.args.valuesFiles, "valuesFiles", "f", r.args.valuesFiles, "Values file(s) used to template manifests (for now list default values file FIRST)")
	cmd.Flags().StringVar(&r.args.templateFile, "templateFile", r.args.templateFile, "Calico template file to render")
	cmd.MarkFlagRequired("templateFile")
	cmd.Flags().StringVarP(&r.args.outputLocation, "outputLocation", "o", r.args.outputLocation, "File in which to save the outputted calico manifest")

	return cmd.Execute()
}

func (r *runner) start() error {
	// translate the values file into a map
	values, err := r.mapValues()
	if err != nil {
		return err
	}
	r.values = values

	// Take the values file and look for template spots
	templatedCalicoLines, err := r.findAndReplace()
	if err != nil {
		return err
	}

	if err := r.writeManifestFile(templatedCalicoLines); err != nil {
		return err
	}
	return nil
}

// TODO  we expect values in additional values files to override values in default values.yaml?
// Fix this in next version.
// right now it overwrites with whatever is in subsequent values files.
// Only looks top level- i.e if one values file has global.key1: value1, and second has global.key2: value2,
// we only will see global.key2: value2, not both
func (r *runner) mapValues() (map[string]interface{}, error) {
	values := make(map[string](interface{}))
	for _, f := range r.args.valuesFiles {
		fileValues := make(map[string]interface{})
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading values file %s", f)
		}
		err = yaml.Unmarshal(b, &fileValues)
		if err != nil {
			return nil, errors.Wrap(err, "error unmarshalling yaml from values file")
		}
		for k, v := range fileValues {
			values[k] = v
		}
	}
	return values, nil
}

func (r *runner) findAndReplace() ([]string, error) {
	lines, err := readLines(r.args.templateFile)
	if err != nil {
		return nil, errors.Wrap(err, "error reading calico template file")
	}	

	fillMeIn := regexp.MustCompile(`{{(.*)}}`)
	newLines := []string{}
	for _, line := range lines {
		if loc := fillMeIn.FindStringSubmatchIndex(line); loc != nil { // returns [starting index of regex, end index of regex, start index of submatch, end index of submatch]
			if len(loc) < 4 {
				return nil, errors.New(fmt.Sprintf("format error in line %s", line))
			}
			newLine, err := r.replaceTemplateValue(line, loc)
			if err != nil {
				return nil, err
			}
			newLines = append(newLines, newLine)
		} else {
			newLines = append(newLines, line)
		}
	}
	return newLines, nil
}

func readLines(valueFile string) ([]string, error) {
	lines := []string{}
	file, err := os.Open(valueFile)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, err
			}
		}
		// if the line is empty or commented out, don't add
		if ((line != "\n") && !regexp.MustCompile(`^\s{0,}#.*`).Match([]byte(line))) {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func (r *runner) replaceTemplateValue(line string, locationMatch []int) (string, error) {
	valueToGet := strings.TrimSpace(line[locationMatch[2]:locationMatch[3]])
	keys := strings.Split(valueToGet, ".")[1:] // first one will be the context (blank or $)
	if len(keys) < 2 {
		return "", errors.New(fmt.Sprintf("cannot template value %s", valueToGet))
	}
	newValue := ""
	// TODO could be a list, not string or map
	if keys[0] == "Release" {
		switch keys[1] {
		case "Name":
			newValue = r.args.release
		case "Namespace":
			newValue = r.args.namespace
		default: // TODO more release values?
			return "", errors.New(fmt.Sprintf("unknown value: %s", valueToGet))
		}
	} else if keys[0] == "Values" {
		// TODO this is disgusting. and only goes down 3 levels.
		switch len(keys) {
		case 2:
			if s, ok := r.values[keys[1]].(string); ok {
				newValue = s
			}
		case 3:
			if m, ok := r.values[keys[1]].(map[interface{}]interface{}); ok {
				if s, ok := m[keys[2]].(string); ok {
					newValue = s
				}
			}
		case 4:
			if m, ok := r.values[keys[1]].(map[interface{}]interface{}); ok {
				if m2, ok := m[keys[2]].(map[interface{}]interface{}); ok {
					if s, ok := m2[keys[3]].(string); ok {
						newValue = s
					}
				}
			}
		default:
			return "", errors.New(fmt.Sprintf("The value %s is nested too deeply for this program to currently handle", valueToGet))
		}
	} else {
		return "", errors.New(fmt.Sprintf("cannot template value %s", valueToGet))
	}
	
	newLine := line[:locationMatch[0]] + newValue + line[locationMatch[1]:]
	return newLine, nil
}

func (r *runner) writeManifestFile(lines []string) error {
	f, err := os.OpenFile(r.args.outputLocation, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrap(err, "error writing manifest file")
	}

	writer := bufio.NewWriter(f)
	for _, line := range lines {
		_, err := writer.WriteString(line)
		if err != nil {
			return errors.Wrap(err, "error writing manifest file")
		}
	}
	writer.Flush()
	f.Close()

	fmt.Println("Wrote manifest to", r.args.outputLocation)
	fmt.Println("apply it with: calicoctl apply -f", r.args.outputLocation)

	return nil
}

func usage() string {
	return "Usage:\n    calico-template <release-name> <namespace> <values-file> <calico-template-file> <manifest-output-file>"
}
