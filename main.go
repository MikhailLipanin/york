package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	path2 "path"
	"path/filepath"
	"slices"
)

var (
	pathToScratches = flag.String("scratches", "default", "Path to a directory with project files and scratches. Defaults to \"default/\"")
	projectName     = flag.String("name", "unnamed", "Set the project baseName. Defaults to \"unnamed\".")
	verboseOut      = flag.Bool("verbose", true, "Use verbose output.")

	pathToProjectStruct = ""
	scratches           []string
)

const (
	projectStructure = "york.json"
)

type node struct {
	parent   *node
	children []*node
	baseName string
	fullName string
}

func main() {
	flag.Parse()

	err := filepath.WalkDir(*pathToScratches, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && path2.Base(path) == projectStructure {
			pathToProjectStruct = path
		}
		scratches = append(scratches, path)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	if len(pathToProjectStruct) == 0 {
		log.Fatalf("york.json file is not provided in %s", *pathToScratches)
	}

	bytes, err := os.ReadFile(pathToProjectStruct)
	if err != nil {
		log.Fatal(err)
	}

	var data map[string]any
	if err = json.Unmarshal(bytes, &data); err != nil {
		log.Fatal(err)
	}

	log.Println("Starting parsing of scratches...")
	root := &node{}
	if err = parse(root, nil, data); err != nil {
		log.Fatal(err)
	}
	log.Println("Parsing completed!")

	//walk(root, 0)

	log.Println("Starting project generation...")
	if err = generate(root); err != nil {
		log.Fatal(err)
	}

	filesChan := make(chan []string, len(scratches))

	go func() {
		if err = populate(root, filesChan); err != nil {
			log.Fatal(err)
		}
		close(filesChan)
	}()

	for file := range filesChan {
		file := file
		go func() {
			dst, err := os.OpenFile(file[0], os.O_WRONLY, os.ModeAppend)
			if err != nil {
				log.Fatal(err)
			}
			defer dst.Close()

			src, err := os.Open(file[1])
			if err != nil {
				log.Fatal(err)
			}
			defer src.Close()

			if _, err := io.Copy(dst, src); err != nil {
				log.Fatal(err)
			}
		}()
	}

	log.Println("Done! :)")
}

func parse(ver, par *node, data any) error {
	ver.parent = par
	if ver.parent != nil {
		if len(ver.parent.fullName) != 0 {
			ver.fullName = ver.parent.fullName + "/" + ver.baseName
		} else {
			ver.fullName = ver.baseName
		}
	}
	switch t := data.(type) {
	// directory
	case map[string]any:
		for key, val := range t {
			child := &node{baseName: key}
			if err := parse(child, ver, val); err != nil {
				return err
			}
			ver.children = append(ver.children, child)
		}
	// directory's content
	case []any:
		for _, val := range t {
			if err := parse(ver, par, val); err != nil {
				return err
			}
		}
	// file
	case string:
		leaf := &node{baseName: t, parent: ver}
		if leaf.parent != nil {
			if len(leaf.parent.fullName) != 0 {
				leaf.fullName = leaf.parent.fullName + "/" + leaf.baseName
			} else {
				leaf.fullName = leaf.baseName
			}
		}
		ver.children = append(ver.children, leaf)
	default:
		return errors.New("unsupported file definition")
	}

	return nil
}

func generate(ver *node) error {
	if len(ver.baseName) > 0 {
		// this is a file
		if len(ver.children) == 0 {
			file, err := os.Create(ver.fullName)
			if err != nil {
				return err
			}
			if err = file.Close(); err != nil {
				return err
			}
		} else {
			// this is a directory
			if err := os.Mkdir(ver.fullName, 0755); err != nil {
				return err
			}
		}
	}
	for _, child := range ver.children {
		if err := generate(child); err != nil {
			return err
		}
	}
	return nil
}

func populate(ver *node, filesChan chan<- []string) error {
	idx := slices.IndexFunc(scratches, func(s string) bool {
		return path2.Base(s) == ver.baseName
	})
	if idx != -1 {
		filesChan <- []string{ver.fullName, scratches[idx]}
	}
	for _, child := range ver.children {
		if err := populate(child, filesChan); err != nil {
			return err
		}
	}
	return nil
}

func walk(ver *node, lvl int) {
	for i := 0; i < lvl; i++ {
		fmt.Print(" ")
	}
	fmt.Printf("%q:%q\n", ver.baseName, ver.fullName)
	for _, ch := range ver.children {
		walk(ch, lvl+1)
	}
}
