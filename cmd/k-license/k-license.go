/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var excludeDirsLocations = []string{
	"external/bazel_tools",
	".git",
	"node_modules",
	"_output",
	"third_party",
	"vendor",
	"verify/boilerplate/test",
}
var GENERATED_GO_MARKERS = [7]string{
	"// Code generated by client-gen. DO NOT EDIT.",
	"// Code generated by controller-gen. DO NOT EDIT.",
	"// Code generated by counterfeiter. DO NOT EDIT.",
	"// Code generated by deepcopy-gen. DO NOT EDIT.",
	"// Code generated by informer-gen. DO NOT EDIT.",
	"// Code generated by lister-gen. DO NOT EDIT.",
	"// Code generated by protoc-gen-go. DO NOT EDIT.",
}

var codeFileExts = map[string]bool{
	".go":    true,
	".c":     true,
	".h":     true,
	".ipynb": true,
	".py":    true,
	".java":  true,
	".cpp":   true,
	".sh":    true,
}

var buildFileExts = map[string]bool{
	"Makefile":   true,
	"Dockerfile": true,
}

type Options struct {
	templatesDir string
	excludeDirs  []string
	path         string
	confirm      bool
}
type templateFileType struct {
	fileExtension    string // store file extension strings like ".sh", "Makefile", etc.
	templateFileName string // store template file names like "boilerplate.sh.txt", etc
}

var templateFileTypes = []templateFileType{
	{".sh", "boilerplate.sh.txt"},
	{"Makefile", "boilerplate.Makefile.txt"},
	{"Dockerfile", "boilerplate.Dockerfile.txt"},
	{".py", "boilerplate.py.txt"},
	{".go", "boilerplate.go.txt"},
}

var opts = &Options{}

func main() {
	rootCmd := &cobra.Command{
		Use:   "k-license",
		Short: "Tool for Adding license Headers",
	}
	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Add Headers to files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run()
		},
	}

	addCmd.Flags().StringVar(&opts.templatesDir, "templates", "../../hack/boilerplate", "directory containing license templates")
	addCmd.Flags().StringSliceVarP(&opts.excludeDirs, "exclude", "e", excludeDirsLocations, "comma-separated list of directories to exclude")
	addCmd.Flags().StringVar(&opts.path, "path", ".", "Defaults to Current directory")
	addCmd.Flags().BoolVar(&opts.confirm, "confirm", false, "confirm actually adding license boilerplate to files")
	rootCmd.AddCommand(addCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

}
func (opts *Options) Run() error {
	files := 0
	fileList := make([]string, 0)
	err := filepath.WalkDir(opts.path, func(path string, info fs.DirEntry, err error) error {
		if info.IsDir() && containsExcluded(opts.excludeDirs, info.Name()) {
			return filepath.SkipDir
		}
		if !info.IsDir() && (isCodeFile(path) || isBuildFile(path)) && !isGenerateFile(path) {
			hasLic, err := hasLicense(path)
			if !hasLic {
				currentYear := strconv.Itoa(time.Now().Year())
				if opts.confirm {
					err := addLicense(path, opts.templatesDir, currentYear)
					if err != nil {
						return err
					}
					fmt.Printf("Modified %s file\n", path)
				}
				fileList = append(fileList, path)
				files++
			}
			return err

		}
		if err != nil {
			return err
		}
		return nil
	})
	if opts.confirm {
		fmt.Printf("Modified %v files\n", files)
	} else {
		fmt.Printf("DRY RUN: No file changes will be made! To make file modifications, rerun the command with  \"--confirm\" flag\n")
		if files == 0 {
			fmt.Printf("All files have appropriate License Headers. No changes required.\n")
		}
		if files > 0 {
			fmt.Printf("%v files will be modified to add License Headers\n", files)
			fmt.Printf("Listing files to be modified:\n")
			for _, file := range fileList {
				fmt.Printf("%s\n", file)
			}
		}

	}
	return err
}

// Looks for the Excluded files/directroy
func containsExcluded(list []string, str string) bool {
	for _, item := range list {
		if item == str {
			fmt.Printf("Skipping %s as this is Part of exclude list\n", str)
			return true
		}
	}
	return false
}

// Check if the file is code File
func isCodeFile(path string) bool {
	return codeFileExts[strings.ToLower(filepath.Ext(path))]
}

// Check if the file is build File
func isBuildFile(path string) bool {
	return buildFileExts[filepath.Base(path)]
}

// Checks if the file is auto generated
func isGenerateFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, ft := range GENERATED_GO_MARKERS {
		if strings.Contains(string(data), ft) {
			fmt.Printf("Skipping File: %s since this is autogenerated file \n", path)
			return true
		}
	}
	return false
}

// Checks for license in the file
func hasLicense(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if strings.Contains(string(data), "Copyright") && strings.Contains(string(data), "Licensed under the Apache License") {
		fmt.Printf("Skipping File: %s Already have templates added\n", path)
		return true, nil
	} else {
		return false, nil
	}
}

// Reads templates from directory
func getTemplateFile(path string) string {
	for _, file := range templateFileTypes {
		if strings.HasSuffix(path, file.fileExtension) || filepath.Base(path) == file.fileExtension {
			return file.templateFileName
		}
	}
	return "boilerplate.tf.txt"
}

// Adds License Headers
func addLicense(path, templatesDir, year string) error {
	tmplData, err := os.ReadFile(filepath.Join(templatesDir, getTemplateFile(path)))
	if err != nil {
		return err
	}
	// Replace placeholders with actual values
	tmpl := strings.ReplaceAll(string(tmplData), "YEAR", year)

	if fileSize(path) {
		codeData, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		newData := append([]byte(tmpl), []byte("\n")...)
		newData = append(newData, codeData...)
		return os.WriteFile(path, newData, 0644)
	}
	return nil
}

// Check for empty file
func fileSize(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		fmt.Println(err)
		return false
	}

	if fileInfo.Size() == 0 {
		fmt.Println("The file is empty No Modification Required")
		return false
	} else {
		fmt.Println("The file is not empty")
		return true
	}
}
