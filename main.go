package main

import (
	"bytes"
	"errors"
	"flag"
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
	var (
		ns, app, file, uri string
		err                error
		b                  []byte
	)
	flag.StringVar(&ns, "n", "default", "namespace")
	flag.StringVar(&app, "a", "", "appname")
	flag.StringVar(&file, "f", "", "file name")
	flag.StringVar(&uri, "u", "", "url")
	flag.Parse()

	// Get bundle
	if file != "" {
		b, err = ioutil.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", file, err)
			os.Exit(1)
		}
	} else if uri != "" {
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
	} else {
		fmt.Fprintf(os.Stderr, "no input\n")
		os.Exit(1)
	}

	// split out files
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
