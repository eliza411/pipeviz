package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"

	"github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/spf13/cobra"
	gjs "github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/xeipuuv/gojsonschema"
	"github.com/tag1consulting/pipeviz/interpret"
	"github.com/tag1consulting/pipeviz/represent"
	"github.com/tag1consulting/pipeviz/schema"
)

func dotDumperCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dotdump [-o|--output <path>] <dir>...",
		Short: "Dumps a graphviz-generated png of some/all of the JSON fixtures.",
		Long: `This tool generates a dot representation of a pipeviz core graph,
		having constructed that graph by merging in all JSON fixtures found in all the specified directories, in lexicographic order.`,
		Run: runDotDumper,
	}

	var all bool
	var path string
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Includes all input message fixtures, in order. Arguments are ignored.")
	cmd.Flags().StringVarP(&path, "output", "o", "", "Specifies a file to write dot output to. Otherwise, prints to stdout.")

	return cmd
}

func runDotDumper(cmd *cobra.Command, args []string) {
	g := represent.NewGraph()
	raw, err := schema.Master()
	if err != nil {
		panic(fmt.Sprint("Failed to open master schema file, test must abort. message:", err.Error()))
	}

	schemaMaster, err := gjs.NewSchema(gjs.NewStringLoader(string(raw)))
	if err != nil {
		panic("bad schema...?")
	}

	if len(args) < 1 {
		log.Fatalf("Must provide at least one directory argument to dotdumper.")
	}

	var k uint64 = 0
	for _, dir := range args {
		fl, err := ioutil.ReadDir(dir)
		if err != nil {
			erro.Printf("Failed to read directory '%v' with error %v\n", dir, err)
		}

		for _, f := range fl {
			if match, _ := regexp.MatchString("\\.json$", f.Name()); match && !f.IsDir() {
				src, err := ioutil.ReadFile(dir + "/" + f.Name())
				if err != nil {
					erro.Printf("Failed to read fixture file %v/%v\n", dir, f.Name())
					continue
				}

				result, err := schemaMaster.Validate(gjs.NewStringLoader(string(src)))
				if err != nil {
					erro.Printf("Validation process terminated with errors for %v/%v. Error: \n%v\n", dir, f.Name(), err.Error())
					continue
				}

				if !result.Valid() {
					for _, desc := range result.Errors() {
						erro.Printf("\t%s\n", desc)
					}
				} else {
					k++
					m := interpret.Message{Id: k}
					json.Unmarshal(src, &m)

					g = g.Merge(m)
					fmt.Printf("Merged message %v/%v into graph\n", dir, f.Name())
				}
			}
		}
	}

	pathflag := cmd.Flags().Lookup("output")
	if pathflag.Changed {
		ioutil.WriteFile(pathflag.Value.String(), represent.GenerateDot(g), 0644)
	} else {
		fmt.Println(string(represent.GenerateDot(g)))
	}
}
