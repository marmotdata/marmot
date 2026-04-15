package controller

import sigsyaml "sigs.k8s.io/yaml"

func yamlMarshal(v interface{}) ([]byte, error) {
	return sigsyaml.Marshal(v)
}
