package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/xeipuuv/gojsonschema"
)

func main() {
	nargs := len(os.Args[1:])
	if nargs == 0 || nargs > 2 {
		fmt.Printf("ERROR: usage is: %s <schema.json> [<document.json>]\n", os.Args[0])
		os.Exit(1)
	}

	schemaPath, err := filepath.Abs(os.Args[1])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	schemaLoader := gojsonschema.NewReferenceLoader("file://" + schemaPath)
	var documentLoader gojsonschema.JSONLoader

	if nargs > 1 {
		documentPath, err := filepath.Abs(os.Args[2])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		documentLoader = gojsonschema.NewReferenceLoader("file://" + documentPath)
	} else {
		documentBytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		documentString := string(documentBytes)
		documentLoader = gojsonschema.NewStringLoader(documentString)
	}

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		panic(err.Error())
	}

	if result.Valid() {
		fmt.Printf("The document is valid\n")
	} else {
		fmt.Printf("The document is not valid. see errors :\n")
		for _, desc := range result.Errors() {
			fmt.Printf("- %s\n", desc)
		}
		os.Exit(1)
	}
}
