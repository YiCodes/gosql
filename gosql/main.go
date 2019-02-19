package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/YiCodes/gosql/sqlcodegen"
)

var (
	input, output string
)

func init() {
	flag.StringVar(&input, "in", ".", "source file or directory")
	flag.StringVar(&output, "out", "", "output directory")
}

func makeDir(dir string) error {
	f, err := os.Stat(dir)

	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(dir, os.ModePerm)
		}
	} else {
		if !f.IsDir() {
			return fmt.Errorf("%v is a file", output)
		}
	}

	return err
}

func isDir(path string) bool {
	f, err := os.Stat(path)

	if err == nil {
		return f.IsDir()
	}

	return false
}

func genCode() error {
	var err error

	if !filepath.IsAbs(input) {
		input, err = filepath.Abs(input)

		if err != nil {
			return err
		}
	}

	fmt.Printf("in: %v\n", input)

	if output == "" {
		var d string

		if isDir(input) {
			d = input
		} else {
			d = filepath.Dir(input)
		}

		output = filepath.Join(d, "gen")
	}

	fmt.Printf("out: %v\n", output)

	if !filepath.IsAbs(output) {
		output, _ = filepath.Abs(output)
	}

	err = makeDir(output)

	if err != nil {
		return err
	}

	inputInfo, err := os.Stat(input)

	if err != nil {
		return err
	}

	if inputInfo.IsDir() {
		files, err := ioutil.ReadDir(input)

		if err != nil {
			return err
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}

			src := filepath.Join(input, f.Name())
			dest := filepath.Join(output, f.Name())

			err = sqlcodegen.Compile(src, dest, sqlcodegen.Options{})

			if err != nil {
				os.Remove(dest)

				return err
			}
		}
	} else {
		_, fileName := filepath.Split(input)
		dest := filepath.Join(output, fileName)

		err = sqlcodegen.Compile(input, dest, sqlcodegen.Options{})

		if err != nil {
			os.Remove(dest)

			return err
		}
	}

	fmt.Println("complete.")

	return nil
}

func main() {
	flag.Parse()

	err := genCode()

	if err != nil {
		fmt.Println(err)
	}
}
