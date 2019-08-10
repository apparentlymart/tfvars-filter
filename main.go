package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/hashicorp/hcl2/hcl"
	"github.com/hashicorp/hcl2/hclwrite"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	flag "github.com/spf13/pflag"
	"github.com/zclconf/go-cty/cty"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tfvars-filter <config-dir> <tfvars-file>\n\n")
		flag.PrintDefaults()
	}
	help := flag.BoolP("help", "h", false, "show this help")
	outputFile := flag.StringP("output", "o", "-", "file to write to")
	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(1)
	}

	configDir := args[0]
	tfvarsFilename := args[1]
	var tfvarsSrc []byte
	switch tfvarsFilename {
	case "-":
		var err error
		tfvarsSrc, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read tfvars from stdin: %s\n", err)
			os.Exit(1)
		}
	default:
		var err error
		tfvarsSrc, err = ioutil.ReadFile(tfvarsFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot read tfvars from %s: %s\n", tfvarsFilename, err)
			os.Exit(1)
		}
	}

	mod, configDiags := tfconfig.LoadModule(configDir)
	if configDiags.HasErrors() {
		for _, diag := range configDiags {
			if diag.Pos != nil {
				fmt.Fprintf(os.Stderr, "%s:%d: %s; %s", diag.Pos.Filename, diag.Pos.Line, diag.Summary, diag.Detail)
			} else {
				fmt.Fprintf(os.Stderr, "%s; %s", diag.Summary, diag.Detail)
			}
		}
		os.Exit(1)
	}

	variables := mod.Variables
	var resultSrc []byte

	// TODO: This implementation currently works for native syntax tfvars,
	// but we ought to recognize the .json extension and have a different mode
	// for .tfvars.json too.
	{
		f, varsDiags := hclwrite.ParseConfig(tfvarsSrc, tfvarsFilename, hcl.Pos{Line: 1, Column: 1})
		if varsDiags.HasErrors() {
			for _, diag := range varsDiags {
				if diag.Subject != nil {
					fmt.Fprintf(os.Stderr, "%s:%d: %s; %s", diag.Subject.Filename, diag.Subject.Start.Line, diag.Summary, diag.Detail)
				} else {
					fmt.Fprintf(os.Stderr, "%s; %s", diag.Summary, diag.Detail)
				}
			}
			os.Exit(1)
		}

		body := f.Body()
		for name := range body.Attributes() {
			if _, ok := variables[name]; ok {
				continue
			}
			// TODO: Should actually filter this out, but hclwrite is missing a
			// function for that right now.
			body.SetAttributeValue(name, cty.NullVal(cty.DynamicPseudoType))
		}

		resultSrc = f.Bytes()
	}


	switch *outputFile {
	case "-":
		_, err := os.Stdout.Write(resultSrc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to write result to stdout: %s", err)
		}
	default:
		err := ioutil.WriteFile(*outputFile, resultSrc, os.ModePerm)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to write result to %s: %s", *outputFile, err)
		}
	}
}
