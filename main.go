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
	"gopkg.in/yaml.v2"
)

type args struct {
	release string
	namespace string
	valuesFile string
	templateFile string
	manifestLocation string
}

type runner struct {
	args args
	values map[string]interface{}
}

// TODO there will often be 2 values files
// TODO this assumes that all values look like straight {{ blah }} and not {{ bla | blah }} or {{ include blah }} etc.
// Needs to take in release name, release namespace, values file, calico-templates, and manifest output location
func main() {
	if err := run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func run() error {
	if (len(os.Args[1:]) < 5) {
		return errors.New(fmt.Sprintf("incorrect number of arguments.\n%s", usage()))
	}
	r := &runner{
		args: args{
			release: os.Args[1], // TODO use cobra if we might ever use this from cli. probably good to even if just from CI/CD
			namespace: os.Args[2],
			valuesFile: os.Args[3],
			templateFile: os.Args[4],
			manifestLocation: os.Args[5],
		},
	}

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

func (r *runner) mapValues() (map[string]interface{}, error) {
	valuesBytes, err := ioutil.ReadFile(r.args.valuesFile)
	if err != nil {
		return nil, errors.Wrap(err, "error reading values file")
	}
	values := make(map[string](interface{}))
	err = yaml.Unmarshal(valuesBytes, &values)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling yaml from values file")
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
				fmt.Println("r.values[keys[1]] is a map")
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
	f, err := os.OpenFile(r.args.manifestLocation, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

	fmt.Println("Wrote manifest to", r.args.manifestLocation)
	fmt.Println("apply it with: calicoctl apply -f", r.args.manifestLocation)

	return nil
}

func usage() string {
	return "Usage:\n    calico-template <release-name> <namespace> <values-file> <calico-template-file> <manifest-output-file>"
}
