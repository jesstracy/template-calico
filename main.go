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
}

type runner struct {
	args args
}

// TODO there will often be 2 values files
// TODO this assumes that all values look like straight {{ blah }} and not {{ bla | blah }} or {{ include blah }} etc.
// Needs to take in release name, release namespace, values file, and calico-templates
func main() {
	if err := run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func run() error {
	if (len(os.Args[1:]) < 4) {
		return errors.New(fmt.Sprintf("incorrect number of arguments.\n%s", usage()))
	}
	r := &runner{
		args: args{
			release: os.Args[1],
			namespace: os.Args[2],
			valuesFile: os.Args[3],
			templateFile: os.Args[4],
		},
	}

	// translate the values file into a map
	values, err := r.mapValues()
	if err != nil {
		return err
	}

	for k, v := range values {
		fmt.Println(k, v)
	}

	// Take the values file and look for template spots
	templatedCalicoLines, err := r.findAndReplace()
	//_, err = r.findAndReplace()
	if err != nil {
		return err
	}

	fmt.Println("\ntemplated manifest:")
	for _, line := range templatedCalicoLines {
		fmt.Println(line)
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
	// read in lines from calico template
	lines, err := readLines(r.args.templateFile)
	if err != nil {
		return nil, errors.Wrap(err, "error reading calico template file")
	}	
	// look for stuff that starts with {{
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

// TODO unit test
func (r *runner) replaceTemplateValue(line string, locationMatch []int) (string, error) {
	fmt.Println("This line has something to replace!!!", line)
	valueToGet := strings.TrimSpace(line[locationMatch[2]:locationMatch[3]])
	keys := strings.Split(valueToGet, ".")[1:] // first one will be the context (blank or $)
	if len(keys) < 2 {
		return "", errors.New(fmt.Sprintf("cannot template value %s", valueToGet))
	}
	newValue := ""
	switch keys[0] {
	case "Release":
		if keys[1] == "Name" {
			newValue = r.args.release
		} else if keys[1] == "Namespace" {
			newValue = r.args.namespace
		} else { // TODO more release things?
			return "", errors.New(fmt.Sprintf("unknown value: %s", valueToGet))
		}
	case "Values":
		newValue = "We got a VALUE ohno"
	default:
		return "", errors.New(fmt.Sprintf("cannot template value %s", valueToGet))
	}
	newLine := line[:locationMatch[0]] + newValue + line[locationMatch[1]:]
	return newLine, nil
}

func usage() string {
	return "Usage:\n    calico-template <release-name> <namespace> <values-file> <calico-template-file>"
}
