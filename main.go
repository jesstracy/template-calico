package main

import (
	"fmt"
	"io/ioutil"
	"os"
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
	values, err := r.pullValues()
	if err != nil {
		return err
	}

	for k, v := range values {
		fmt.Println(k, v)
	}

	// Take the values file and look for template spots
	//templatedValuesFile, err := r.findAndReplace()
	//if err != nil {
	//	return err
	//}
	return nil
}

func (r *runner) pullValues() (map[string]interface{}, error) {
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

func (r *runner) findAndReplace() (string, error) {
	// look for stuff that starts with {{
	return "", nil
}

func usage() string {
	return "Usage:\n    calico-template <release-name> <namespace> <values-file> <calico-template-file>"
}
