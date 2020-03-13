package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"text/tabwriter"

	"sigs.k8s.io/yaml"
)

type Namespace struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"metadata"`
	} `json:"metadata"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s appname [FILE | URL]\n", os.Args[0])
		os.Exit(1)
	}
	app, uri := os.Args[1], os.Args[2]

	// Get bundle
	b, err := ioutil.ReadFile(os.Args[1])
	if perr := (&os.PathError{}); errors.As(err, &perr) {
		r, err := http.Get(uri)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GET %s: %v\n", uri, err)
			os.Exit(1)
		}
		b, err = ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "read response: %v\n", err)
			os.Exit(1)
		}
	}

	// split out files
	var ns string
	bb := bytes.Split(b, []byte("\n---\n"))
	kinds := map[string][][]byte{}
	for i := range bb {
		if len(bytes.TrimSpace(bb[i])) == 0 {
			continue
		}
		var m map[string]interface{}
		err = yaml.Unmarshal(bb[i], &m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unmarshal yaml: %v\n%s\n", err, bb[i])
			os.Exit(1)
		}
		kind := m["kind"].(string)
		if kind == "Namespace" {
			ns = m["metadata"].(map[string]interface{})["name"].(string)
		}
		kinds[kind] = append(kinds[kind], bb[i])
	}
	delete(kinds, "Namespace")

	// write out files
	dirs := fmt.Sprintf("./%s/%s", ns, app)
	err = os.MkdirAll(dirs, 0o755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkdirall %s: %v\n", dirs, err)
		os.Exit(1)
	}

	fn := func(kind string) string {
		return fmt.Sprintf("./%s/%s/%s.yaml", ns, app, kind)
	}
	var res []string
	for k, bb := range kinds {
		res = append(res, k)
		err = ioutil.WriteFile(fn(k), bytes.Join(bb, []byte("\n---\n")), 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "write yaml %s: %v\n", k, err)
			os.Exit(1)
		}
	}

	sort.Strings(res)

	// root kustomization
	addBase("./kustomization.yaml", "", ns, nil)
	// namespace kustomization
	addBase(fmt.Sprintf("./%s/kustomization.yaml", ns), ns, app, nil)
	// app kustomization
	addBase(fmt.Sprintf("./%s/%s/kustomization.yaml", ns, app), "", "", res)

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
	for _, k := range res {
		fmt.Fprintf(w, "%s\t%d\n", k, len(kinds[k]))
	}
}

func addBase(path, ns, base string, res []string) {
	kustom := map[string]interface{}{}
	b, err := ioutil.ReadFile(path)
	if perr := (&os.PathError{}); errors.As(err, &perr) {
		// noop
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "kustomization read %s: %v\n", path, err)
		os.Exit(1)
	} else {
		err = yaml.Unmarshal(b, &kustom)
		if err != nil {
			fmt.Fprintf(os.Stderr, "kustomization unmarshal %s: %v\n", path, err)
			os.Exit(1)
		}
	}

	bases := []string{base}
	bs, ok := kustom["Bases"].([]string)
	if ok {
		bases = append(bases, bs...)
	}

	if ns != "" {
		kustom["ramespace"] = ns
	}
	if len(bases) != 0 {
		kustom["bases"] = bases
	}
	if len(res) != 0 {
		kustom["resources"] = res
	}

	b, err = yaml.Marshal(kustom)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kustomization marshal %s: %v\n", path, err)
		os.Exit(1)
	}
	err = ioutil.WriteFile(path, b, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kustomization write %s: %v\n", path, err)
		os.Exit(1)
	}

	if ns == "" {
		return
	}

	path = fmt.Sprintf("./%s/Namespace.yaml", ns)
	_, err = os.Stat(path)
	if err == nil {
		return
	}
	b, err = yaml.Marshal(Namespace{
		APIVersion: "v1",
		Kind:       "Namespace",
		Metadata: struct {
			Name string `json:"metadata"`
		}{
			ns,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "namespace marshal %s: %v\n", path, err)
		os.Exit(1)
	}
	err = ioutil.WriteFile(path, b, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "namespace write %s: %v\n", path, err)
		os.Exit(1)
	}
}
